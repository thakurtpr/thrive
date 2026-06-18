//go:build !linux

package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func LogsCmd() *cobra.Command {
	logs := &cobra.Command{
		Use:   "logs [container]",
		Short: "Fetch container logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			containerID := args[0]
			follow, _ := cmd.Flags().GetBool("follow")
			opts := map[string]any{"follow": follow}

			if follow {
				return vm.DialControlStream(ctx, "logs", []string{containerID}, opts, os.Stdout)
			}

			data, err := vm.DialControl(ctx, "logs", []string{containerID}, opts)
			if err != nil {
				return fmt.Errorf("logs failed: %w", err)
			}

			// Response format: {"output":"log content here\n"}
			var result map[string]any
			if jsonErr := json.Unmarshal(data, &result); jsonErr == nil {
				if output, ok := result["output"].(string); ok {
					fmt.Print(output)
					return nil
				}
			}
			fmt.Print(string(data))
			return nil
		},
	}

	logs.Flags().BoolP("follow", "f", false, "Follow log output")
	return logs
}
