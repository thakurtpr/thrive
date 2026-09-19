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
			if since, _ := cmd.Flags().GetString("since"); since != "" {
				return fmt.Errorf("logs: --since requires per-line timestamps, which thrive's daemonless log files do not record")
			}
			if until, _ := cmd.Flags().GetString("until"); until != "" {
				return fmt.Errorf("logs: --until requires per-line timestamps, which thrive's daemonless log files do not record")
			}
			if ts, _ := cmd.Flags().GetBool("timestamps"); ts {
				return fmt.Errorf("logs: --timestamps requires per-line timestamps, which thrive's daemonless log files do not record")
			}
			ctx := cmd.Context()
			containerID := args[0]
			follow, _ := cmd.Flags().GetBool("follow")
			tail, _ := cmd.Flags().GetInt("tail")
			opts := map[string]any{"follow": follow}
			if tail > 0 {
				opts["tail"] = tail
			}

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
	logs.Flags().Int("tail", 0, "Number of lines to show from the end of the logs (0 = all)")
	logs.Flags().String("since", "", "Show logs since timestamp (not supported: no per-line timestamps recorded)")
	logs.Flags().String("until", "", "Show logs before timestamp (not supported: no per-line timestamps recorded)")
	logs.Flags().BoolP("timestamps", "t", false, "Show timestamps (not supported: no per-line timestamps recorded)")
	return logs
}
