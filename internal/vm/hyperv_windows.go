//go:build windows

package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

type hyperVBridge struct {
	conn net.Conn
}

func newHyperVBridge() (Bridge, error) {
	// Connect to Hyper-V VM via named pipe
	pipePath := `\\.\pipe\thrive-daemon`

	conn, err := winio.DialPipe(pipePath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Hyper-V VM: %w", err)
	}

	return &hyperVBridge{conn: conn}, nil
}

func (b *hyperVBridge) Exec(ctx context.Context, cmd string, args []string, opts map[string]any) ([]byte, error) {
	req := map[string]any{"cmd": cmd, "args": args, "opts": opts}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	b.conn.SetDeadline(time.Now().Add(30 * time.Second))

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
