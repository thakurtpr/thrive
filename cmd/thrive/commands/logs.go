//go:build linux

package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/runtime"
)

func LogsCmd() *cobra.Command {
	var follow bool
	var tail int

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

			// Dump existing content (last N lines with --tail).
			if tail > 0 {
				data, err := io.ReadAll(f)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error reading logs: %v\n", err)
					os.Exit(1)
				}
				lines := splitLines(string(data))
				if len(lines) > tail {
					lines = lines[len(lines)-tail:]
				}
				for _, l := range lines {
					fmt.Println(l)
				}
			} else if _, err := io.Copy(os.Stdout, f); err != nil {
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
	cmd.Flags().IntVar(&tail, "tail", 0, "Number of lines to show from the end of the logs (0 = all)")
	return cmd
}

func splitLines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
