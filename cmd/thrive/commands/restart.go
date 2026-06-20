//go:build linux

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/runtime"
)

func RestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart [container]",
		Short: "Restart a container",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			id := args[0]

			state, err := runtime.State(ctx, id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: container not found: %v\n", err)
				os.Exit(1)
			}

			if state.Status == "running" {
				runtime.Kill(ctx, id, syscall.SIGTERM)
				deadline := time.Now().Add(10 * time.Second)
				for time.Now().Before(deadline) {
					time.Sleep(200 * time.Millisecond)
					s, err := runtime.State(ctx, id)
					if err != nil || s.Status == "stopped" {
						break
					}
				}
				runtime.Kill(ctx, id, syscall.SIGKILL)
			}

			// Reset state and restart
			containerDir := filepath.Join("/run/thrive/containers", id)
			stateData, _ := os.ReadFile(filepath.Join(containerDir, "state.json"))
			var s runtime.ContainerState
			json.Unmarshal(stateData, &s)
			s.Status = "created"
			data, _ := json.Marshal(s)
			os.WriteFile(filepath.Join(containerDir, "state.json"), data, 0644)

			if _, err := runtime.Start(ctx, id); err != nil {
				fmt.Fprintf(os.Stderr, "Error restarting container: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("container %s restarted\n", id)
		},
	}
}
