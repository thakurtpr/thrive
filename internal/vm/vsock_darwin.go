//go:build darwin

package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"time"
)

type vsockBridge struct {
	conn net.Conn
}

// newVSOCKBridge connects to vfkit's virtio-vsock Unix socket proxy.
// vfkit creates vsock.sock after the VM starts; we retry for up to 5 seconds.
func newVSOCKBridge() (Bridge, error) {
	sockPath := filepath.Join(ThriveDir(), "vm", "vsock.sock")
	var (
		conn net.Conn
		err  error
	)
	for i := 0; i < 10; i++ {
		conn, err = net.DialTimeout("unix", sockPath, time.Second)
		if err == nil {
			return &vsockBridge{conn: conn}, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil, fmt.Errorf("vsock bridge: %s not ready: %w", sockPath, err)
}

func (b *vsockBridge) Exec(ctx context.Context, cmd string, args []string, opts map[string]any) ([]byte, error) {
	req := map[string]any{"cmd": cmd, "args": args, "opts": opts}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	if err := b.conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return nil, err
	}

	if _, err := b.conn.Write(append(data, '\n')); err != nil {
		return nil, err
	}

	respData, err := io.ReadAll(b.conn)
	if err != nil {
		return nil, err
	}

	var resp map[string]any
	if err := json.Unmarshal(respData, &resp); err != nil {
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

	if _, err := b.conn.Write(append(data, '\n')); err != nil {
		return err
	}

	decoder := json.NewDecoder(b.conn)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var resp map[string]any
		if err := decoder.Decode(&resp); err != nil {
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