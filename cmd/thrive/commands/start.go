//go:build linux

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/runtime"
)

func StartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start [container]",
		Short: "Start a stopped container",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			id := args[0]

			containerDir := filepath.Join("/run/thrive/containers", id)
			stateData, err := os.ReadFile(filepath.Join(containerDir, "state.json"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: container not found: %v\n", err)
				os.Exit(1)
			}

			var state runtime.ContainerState
			if err := json.Unmarshal(stateData, &state); err != nil {
				fmt.Fprintf(os.Stderr, "Error reading state: %v\n", err)
				os.Exit(1)
			}
			if state.Status == "running" {
				fmt.Printf("container %s is already running\n", id)
				return
			}

			state.Status = "created"
			data, _ := json.Marshal(state)
			os.WriteFile(filepath.Join(containerDir, "state.json"), data, 0644)

			if _, err := runtime.Start(ctx, id); err != nil {
				fmt.Fprintf(os.Stderr, "Error starting container: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("container %s started\n", id)
		},
	}
}
