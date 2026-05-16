//go:build linux

package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/runtime"
)

func LogsCmd() *cobra.Command {
	var follow bool

	cmd := &cobra.Command{
		Use:   "logs [container]",
		Short: "View container logs",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			containerID := args[0]

			// Verify container exists.
			if _, err := runtime.State(ctx, containerID); err != nil {
				fmt.Fprintf(os.Stderr, "Error: container %q not found: %v\n", containerID, err)
				os.Exit(1)
			}

			logPath := filepath.Join("/run/thrive/containers", containerID, "logs")
			f, err := os.Open(logPath)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintf(os.Stderr, "No logs yet for container %s\n", containerID)
					os.Exit(0)
				}
				fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
				os.Exit(1)
			}
			defer f.Close()

			// Dump existing content.
			if _, err := io.Copy(os.Stdout, f); err != nil {
				fmt.Fprintf(os.Stderr, "Error reading logs: %v\n", err)
				os.Exit(1)
			}

			if !follow {
				return
			}

			// --follow: poll for new bytes until the container stops.
			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()

			for range ticker.C {
				if _, err := io.Copy(os.Stdout, f); err != nil {
					return
				}
				state, err := runtime.State(ctx, containerID)
				if err != nil || state.Status == "stopped" {
					return
				}
			}
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	return cmd
}
