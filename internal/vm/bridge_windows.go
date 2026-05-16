//go:build windows

package vm

import (
	"context"
	"fmt"
	"io"
)

// Hyper-V bridge for Windows
type hypervBridge struct{}

func newHyperVBridge() (Bridge, error) {
	return &hypervBridge{}, nil
}

func (b *hypervBridge) Exec(ctx context.Context, cmd string, args []string, opts map[string]any) ([]byte, error) {
	return nil, fmt.Errorf("hyperv bridge not implemented")
}

func (b *hypervBridge) ExecStream(ctx context.Context, cmd string, args []string, opts map[string]any, out io.Writer) error {
	return fmt.Errorf("hyperv bridge not implemented")
}

func (b *hypervBridge) Close() error {
	return nil
}

// WSL2 bridge for Windows
type wsl2Bridge struct{}

func newWSL2Bridge() (Bridge, error) {
	return &wsl2Bridge{}, nil
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