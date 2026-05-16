//go:build !linux

package vm

import (
	"context"
	"io"
)

// Bridge is the transport abstraction for CLI→VM daemon communication.
// Implemented per-platform: VSOCK (macOS), named pipe (Windows Hyper-V), WSL2 interop (Windows).
type Bridge interface {
	// Exec runs a Thrive CLI command inside the VM and returns structured output.
	// opts carries additional parameters (e.g., {"follow": true} for logs).
	Exec(ctx context.Context, cmd string, args []string, opts map[string]any) ([]byte, error)

	// ExecStream runs a streaming command (logs --follow) writing output to out as lines arrive.
	// Returns when {"eof": true} is received or ctx is cancelled.
	ExecStream(ctx context.Context, cmd string, args []string, opts map[string]any, out io.Writer) error

	Close() error
}