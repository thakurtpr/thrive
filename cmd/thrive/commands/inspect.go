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

func InspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect [container]",
		Short: "Display detailed information about a container",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			id := args[0]

			state, err := runtime.State(ctx, id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: container not found: %v\n", err)
				os.Exit(1)
			}

			configPath := filepath.Join("/run/thrive/containers", id, "config.json")
			configData, _ := os.ReadFile(configPath)
			var cfg map[string]any
			json.Unmarshal(configData, &cfg)

			info := map[string]any{
				"id":     state.ID,
				"status": state.Status,
				"pid":    state.PID,
				"config": cfg,
			}

			out, err := json.MarshalIndent(info, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(out))
		},
	}
}
