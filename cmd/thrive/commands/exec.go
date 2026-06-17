//go:build linux

package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/runtime"
)

func ExecCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "exec [container] [command...]",
		Short: "Execute a command in a running container",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			containerID := args[0]
			command := args[1:]

			state, err := runtime.State(ctx, containerID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: container not found: %v\n", err)
				os.Exit(1)
			}
			if state.Status != "running" || state.PID == 0 {
				fmt.Fprintf(os.Stderr, "Error: container %s is not running\n", containerID)
				os.Exit(1)
			}

			nsenterArgs := []string{
				"--target", strconv.Itoa(state.PID),
				"--mount", "--pid", "--ipc", "--uts", "--net",
				"--",
			}
			nsenterArgs = append(nsenterArgs, command...)

			execCmd := exec.CommandContext(ctx, "nsenter", nsenterArgs...)
			execCmd.Stdin = os.Stdin
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr

			if err := execCmd.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
						os.Exit(ws.ExitStatus())
					}
				}
				fmt.Fprintf(os.Stderr, "exec error: %v\n", err)
				os.Exit(1)
			}
		},
	}
}
