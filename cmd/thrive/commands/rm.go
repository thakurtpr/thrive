//go:build linux

package commands

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/runtime"
)

func RmCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rm [container]",
		Short: "Remove a container",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			containerID := args[0]

			// Docker parity (mirrors compose.Rm and the VM daemon): refuse
			// to remove running containers unless forced.
			if state, err := runtime.State(ctx, containerID); err == nil && state.Status == "running" {
				if !force {
					fmt.Fprintf(os.Stderr, "Error: container %s is running (use --force)\n", containerID)
					os.Exit(1)
				}
				if err := runtime.Kill(ctx, containerID, syscall.SIGKILL); err != nil {
					fmt.Fprintf(os.Stderr, "Error killing container: %v\n", err)
					os.Exit(1)
				}
			}

			// Unmount image
			if err := image.Unmount(ctx, containerID); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: unmount error: %v\n", err)
			}

			// Delete container state
			if err := runtime.Delete(ctx, containerID); err != nil {
				fmt.Fprintf(os.Stderr, "Error removing container: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Container %s removed\n", containerID)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Remove a running container (kills first)")
	return cmd
}
