//go:build darwin

package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

// vsockListener is created before vfkit spawns so vfkit's startup probe
// (which fires ~1 second after boot when the guest kernel initialises
// virtio-vsock) succeeds. Kept package-level so PrepareVSOCKListener and
// newVSOCKBridge share state without threading through the call stack.
var vsockListener net.Listener

// PrepareVSOCKListener creates the vsock Unix socket and starts listening
// before vfkit is spawned. Must be called from darwinLauncher.Start before
// exec'ing vfkit, otherwise vfkit logs "connection refused" during its
// startup probe and the vsock proxy is permanently broken.
func PrepareVSOCKListener() error {
	sockPath := filepath.Join(ThriveDir(), "vm", "vsock.sock")
	_ = os.Remove(sockPath)
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("vsock: prepare listener %s: %w", sockPath, err)
	}
	vsockListener = ln
	return nil
}

// CloseVSOCKListener closes and discards the stored listener (e.g. on Stop).
func CloseVSOCKListener() {
	if vsockListener != nil {
		vsockListener.Close()
		vsockListener = nil
	}
}

type vsockBridge struct {
	conn net.Conn
	dec  *json.Decoder
}

// newVSOCKBridge accepts one connection from the vsock listener.
// Auto-creates the listener if needed — thrived is in a reconnect loop and
// connects within ~1 second of the socket appearing.
func newVSOCKBridge() (Bridge, error) {
	if vsockListener == nil {
		if err := PrepareVSOCKListener(); err != nil {
			return nil, fmt.Errorf("vsock: auto-prepare: %w", err)
		}
	}

	_ = vsockListener.(*net.UnixListener).SetDeadline(time.Now().Add(5 * time.Second))
	conn, err := vsockListener.Accept()
	if err != nil {
		vsockListener.Close()
		vsockListener = nil
		return nil, fmt.Errorf("vsock: accept: %w", err)
	}

	return &vsockBridge{conn: conn, dec: json.NewDecoder(conn)}, nil
}

func (b *vsockBridge) Exec(ctx context.Context, cmd string, args []string, opts map[string]any) ([]byte, error) {
	req := map[string]any{"cmd": cmd, "args": args, "opts": opts}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	if err := b.conn.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return nil, err
	}
	if _, err := b.conn.Write(append(data, '\n')); err != nil {
		return nil, err
	}

	// 10 minutes — pull/run operations can take several minutes for large images
	if err := b.conn.SetReadDeadline(time.Now().Add(10 * time.Minute)); err != nil {
		return nil, err
	}

	var resp map[string]any
	if err := b.dec.Decode(&resp); err != nil {
		return nil, err
	}

	if errMsg, ok := resp["error"].(map[string]any); ok {
		return nil, fmt.Errorf("daemon error: %s", errMsg["message"])
	}

	return json.Marshal(resp["result"])
}

func (b *vsockBridge) ExecStream(ctx context.Context, cmd string, args []string, opts map[string]any, out io.Writer) error {
	req := map[string]any{"cmd": cmd, "args": args, "opts": opts}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	if err := b.conn.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return err
	}
	if _, err := b.conn.Write(append(data, '\n')); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := b.conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
			return err
		}

		var resp map[string]any
		if err := b.dec.Decode(&resp); err != nil {
			return err
		}

		if resp["eof"] == true {
			return nil
		}

		if stream, ok := resp["stream"].(string); ok {
			if _, err := out.Write([]byte(stream + "\n")); err != nil {
				return err
			}
		}
	}
}

func (b *vsockBridge) Close() error {
	if b.conn != nil {
		return b.conn.Close()
	}
	return nil
}
