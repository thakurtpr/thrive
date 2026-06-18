//go:build linux

package main

import (
	"context"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
)

func vsockPort() uint32 {
	if p := os.Getenv("THRIVE_VSOCK_PORT"); p != "" {
		v, _ := strconv.ParseUint(p, 10, 32)
		return uint32(v)
	}
	return 1024
}

// rawConn wraps *os.File as net.Conn using blocking kernel I/O.
// This sidesteps Go's netpoller (epoll), which triggers immediate EPOLLHUP
// on AF_VSOCK fds tunneled through vfkit's vsock proxy, causing spurious EOF.
type rawConn struct{ f *os.File }

func (c *rawConn) Read(b []byte) (int, error)         { return c.f.Read(b) }
func (c *rawConn) Write(b []byte) (int, error)        { return c.f.Write(b) }
func (c *rawConn) Close() error                       { return c.f.Close() }
func (c *rawConn) LocalAddr() net.Addr                { return nil }
func (c *rawConn) RemoteAddr() net.Addr               { return nil }
func (c *rawConn) SetDeadline(_ time.Time) error      { return nil }
func (c *rawConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *rawConn) SetWriteDeadline(_ time.Time) error { return nil }

// serveVsock connects to the host via AF_VSOCK (CID_HOST=2, port) and serves
// RPC requests. vfkit's virtio-vsock device proxies each guest connect() to
// the host's Unix socket (vsock.sock).
//
// A fresh socket is created on every attempt — a connected STREAM socket that
// has had a failed Connect cannot be reconnected on the same fd.
func serveVsock(ctx context.Context, port uint32) {
	sa := &unix.SockaddrVM{CID: unix.VMADDR_CID_HOST, Port: port}

	attempt := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		attempt++
		fd, err := unix.Socket(unix.AF_VSOCK, unix.SOCK_STREAM, 0)
		if err != nil {
			if attempt == 1 || attempt%60 == 0 {
				log.Printf("thrived: vsock socket: %v (attempt %d)", err, attempt)
			}
			time.Sleep(time.Second)
			continue
		}

		// Force blocking mode — Go's netpoller (epoll) immediately reports
		// EPOLLHUP on vsock fds through vfkit's proxy, producing spurious EOF.
		unix.SetNonblock(fd, false)

		if err := unix.Connect(fd, sa); err != nil {
			unix.Close(fd)
			if attempt == 1 || attempt%60 == 0 {
				log.Printf("thrived: vsock connect port %d: %v (attempt %d)", port, err, attempt)
			}
			time.Sleep(time.Second)
			continue
		}

		log.Printf("thrived: connected to host on vsock port %d (attempt %d)", port, attempt)

		// Use blocking os.File I/O — net.FileConn breaks on vsock fds via vfkit.
		conn := &rawConn{f: os.NewFile(uintptr(fd), "vsock")}
		handleConn(conn, ctx)

		log.Printf("thrived: vsock connection closed, reconnecting...")
		// No sleep — reconnect immediately so the host can re-accept within ms.
	}
}
