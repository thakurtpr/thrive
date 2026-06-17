//go:build linux

// On Linux containers run natively — no VM proxy needed.
package vm

import (
	"context"
	"fmt"
	"io"
)

func ControlSocketPath() string { return "" }

func ServeControlSocket(_ context.Context, _ interface{}, _ interface{}) {}

func DialControl(_ context.Context, _ string, _ []string, _ map[string]any) ([]byte, error) {
	return nil, fmt.Errorf("control socket not available on Linux (containers run natively)")
}

func DialControlStream(_ context.Context, _ string, _ []string, _ map[string]any, _ io.Writer) error {
	return fmt.Errorf("control socket not available on Linux (containers run natively)")
}
