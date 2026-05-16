//go:build darwin

package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"
)

type vsockBridge struct {
	conn net.Conn
	cid  uint32
}

const vsockPort uint32 = 62373

func newVSOCKBridge() (Bridge, error) {
	// VSOCK CID 3 is the well-known address for the first VM in Hypervisor.framework
	// Connection goes through Hypervisor.framework's vsock device
	// Full implementation uses github.com/mist64/hv bindings
	return nil, fmt.Errorf("vsock bridge requires mist64/hv integration — see docs")
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

	respData := make([]byte, 4096)
	n, err := b.conn.Read(respData)
	if err != nil {
		return nil, err
	}

	var resp map[string]any
	if err := json.Unmarshal(respData[:n], &resp); err != nil {
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