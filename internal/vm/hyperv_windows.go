//go:build windows

package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"
)

type hyperVBridge struct {
	conn net.Conn
	dec  *json.Decoder
}

// newHyperVBridge accepts one connection from the Hyper-V socket listener.
// Mirrors vsockBridge (vsock_darwin.go): the host listens before the VM
// starts, and thrived (unmodified — cmd/thrived/vsock_linux.go) connects out
// to VMADDR_CID_HOST over plain AF_VSOCK, which Linux's guest kernel routes
// over Hyper-V sockets transparently.
func newHyperVBridge() (Bridge, error) {
	if hvsockListener == nil {
		if err := PrepareHVSOCKListener(vsockPort()); err != nil {
			return nil, fmt.Errorf("hvsock: auto-prepare: %w", err)
		}
	}

	conn, err := acceptHVSOCK(5 * time.Second)
	if err != nil {
		return nil, fmt.Errorf("hvsock: accept: %w", err)
	}

	return &hyperVBridge{conn: conn, dec: json.NewDecoder(conn)}, nil
}

func (b *hyperVBridge) Exec(ctx context.Context, cmd string, args []string, opts map[string]any) ([]byte, error) {
	req := map[string]any{"cmd": cmd, "args": args, "opts": opts}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	b.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if _, err := b.conn.Write(append(data, '\n')); err != nil {
		return nil, err
	}

	// 10 minutes — pull/run operations can take several minutes for large images.
	b.conn.SetReadDeadline(time.Now().Add(10 * time.Minute))

	var resp map[string]any
	if err := b.dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("invalid daemon response: %w", err)
	}

	if errMsg, ok := resp["error"].(map[string]any); ok {
		return nil, fmt.Errorf("daemon error: %s", errMsg["message"])
	}

	return json.Marshal(resp["result"])
}

func (b *hyperVBridge) ExecStream(ctx context.Context, cmd string, args []string, opts map[string]any, out io.Writer) error {
	req := map[string]any{"cmd": cmd, "args": args, "opts": opts}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	b.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if _, err := b.conn.Write(append(data, '\n')); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		b.conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		var resp map[string]any
		if err := b.dec.Decode(&resp); err != nil {
			return err
		}

		if resp["eof"] == true {
			return nil
		}

		if stream, ok := resp["stream"].(string); ok {
			fmt.Fprintln(out, stream)
		}
	}
}

func (b *hyperVBridge) Close() error {
	if b.conn != nil {
		return b.conn.Close()
	}
	return nil
}
