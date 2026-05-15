package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/runtime"
)

func RmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm [container]",
		Short: "Remove a container",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			containerID := args[0]

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
}
