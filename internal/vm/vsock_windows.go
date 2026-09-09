//go:build windows

package vm

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	winio "github.com/Microsoft/go-winio"
)

// hvsockListener is created before the Hyper-V VM starts, mirroring
// vsockListener in vsock_darwin.go: the guest (cmd/thrived/vsock_linux.go,
// unmodified) connects out to VMADDR_CID_HOST shortly after boot, and the
// host must already be listening or the connection is refused.
var hvsockListener *winio.HvsockListener

// vsockPort mirrors cmd/thrived/vsock_linux.go's vsockPort() so host and
// guest agree on the service without any guest-side configuration.
func vsockPort() uint32 {
	if p := os.Getenv("THRIVE_VSOCK_PORT"); p != "" {
		if v, err := strconv.ParseUint(p, 10, 32); err == nil {
			return uint32(v)
		}
	}
	return 1024
}

// PrepareHVSOCKListener opens a Hyper-V socket listener for the given vsock
// port before the VM is started. Must be called before Start-VM, otherwise
// the guest's first connect attempt (fired seconds after boot) finds nothing
// listening and thrived falls back to its 1s reconnect loop, delaying boot
// detection unnecessarily.
func PrepareHVSOCKListener(port uint32) error {
	addr := &winio.HvsockAddr{
		VMID:      winio.HvsockGUIDChildren(),
		ServiceID: winio.VsockServiceID(port),
	}
	ln, err := winio.ListenHvsock(addr)
	if err != nil {
		return fmt.Errorf("hvsock: listen on port %d: %w", port, err)
	}
	hvsockListener = ln
	return nil
}

// CloseHVSOCKListener closes and discards the stored listener (e.g. on Stop).
func CloseHVSOCKListener() {
	if hvsockListener != nil {
		hvsockListener.Close()
		hvsockListener = nil
	}
}

// acceptHVSOCK waits up to timeout for the guest to connect. HvsockListener
// has no native deadline support, so the wait is bounded with a goroutine;
// on timeout or error the listener is closed and discarded (Close() reliably
// unblocks the abandoned Accept), forcing the next attempt to prepare a
// fresh listener — mirroring vsockBridge's accept-failure handling.
func acceptHVSOCK(timeout time.Duration) (net.Conn, error) {
	ln := hvsockListener
	if ln == nil {
		return nil, fmt.Errorf("hvsock: no listener prepared")
	}

	type result struct {
		conn net.Conn
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		conn, err := ln.Accept()
		ch <- result{conn, err}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			CloseHVSOCKListener()
			return nil, r.err
		}
		return r.conn, nil
	case <-time.After(timeout):
		CloseHVSOCKListener()
		return nil, fmt.Errorf("accept timed out after %s", timeout)
	}
}
