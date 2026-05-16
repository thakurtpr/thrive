//go:build windows

package vm

import (
	"context"
	"fmt"
	"io"
)

type wsl2Bridge struct{}

func newWSL2Bridge() (Bridge, error) {
	// WSL2 interop exposes Unix sockets to Windows via \\wsl$\instance-name\path
	// Connect to: \\wsl$\Thrive\var\run\thrive-daemon.sock
	return nil, fmt.Errorf("wsl2 bridge requires winio.DialPipe over WSL interop — stub")
}

func (b *wsl2Bridge) Exec(ctx context.Context, cmd string, args []string, opts map[string]any) ([]byte, error) {
	return nil, fmt.Errorf("wsl2 bridge not implemented")
}

func (b *wsl2Bridge) ExecStream(ctx context.Context, cmd string, args []string, opts map[string]any, out io.Writer) error {
	return fmt.Errorf("wsl2 bridge not implemented")
}

func (b *wsl2Bridge) Close() error {
	return nil
}