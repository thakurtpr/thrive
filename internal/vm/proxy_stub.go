//go:build !darwin && !linux

package vm

import (
	"context"
	"fmt"
	"io"
)

func ControlSocketPath() string { return "" }

func ServeControlSocket(_ context.Context, _ *Config, _ interface{}) {}

func DialControl(ctx context.Context, cmd string, args []string, opts map[string]any) ([]byte, error) {
	return nil, fmt.Errorf("control socket not supported on this platform")
}

func DialControlStream(ctx context.Context, cmd string, args []string, opts map[string]any, out io.Writer) error {
	return fmt.Errorf("control socket not supported on this platform")
}
