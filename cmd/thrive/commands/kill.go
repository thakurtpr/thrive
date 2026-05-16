//go:build linux

package commands

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/runtime"
)

func KillCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kill [container]",
		Short: "Kill a container",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			containerID := args[0]

			if err := runtime.Kill(ctx, containerID, syscall.SIGKILL); err != nil {
				fmt.Fprintf(os.Stderr, "Error killing container: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Container %s killed\n", containerID)
		},
	}
}
