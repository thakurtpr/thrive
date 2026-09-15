//go:build !linux

package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

// SystemCmd proxies system operations to the VM daemon.
// Bare `thrive system` still prints info (back-compat).
func SystemCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "Show Thrive system information",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "system", nil, nil)
			if err != nil {
				return fmt.Errorf("system info failed: %w", err)
			}
			fmt.Print(string(data))
			return nil
		},
	}

	df := &cobra.Command{
		Use:   "df",
		Short: "Show disk usage",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "system-df", nil, nil)
			if err != nil {
				return fmt.Errorf("system df failed: %w", err)
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	eventsCmd := &cobra.Command{
		Use:   "events",
		Short: "Stream engine events",
		RunE: func(cmd *cobra.Command, args []string) error {
			since, _ := cmd.Flags().GetString("since")
			until, _ := cmd.Flags().GetString("until")
			filter, _ := cmd.Flags().GetString("filter")
			follow, _ := cmd.Flags().GetBool("follow")
			data, err := vm.DialControl(cmd.Context(), "system-events", nil, map[string]any{
				"since": since, "until": until, "filter": filter, "follow": follow,
			})
			if err != nil {
				return fmt.Errorf("system events failed: %w", err)
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			if evts, ok := result["events"].([]any); ok {
				for _, e := range evts {
					out, _ := json.Marshal(e)
					fmt.Println(string(out))
				}
			}
			return nil
		},
	}
	eventsCmd.Flags().String("since", "", "Show events since timestamp or duration")
	eventsCmd.Flags().String("until", "", "Show events until timestamp or duration")
	eventsCmd.Flags().String("filter", "", "Filter by type")
	eventsCmd.Flags().BoolP("follow", "f", false, "Follow event output")

	prune := &cobra.Command{
		Use:   "prune",
		Short: "Remove unused data",
		RunE: func(cmd *cobra.Command, args []string) error {
			all, _ := cmd.Flags().GetBool("all")
			volumes, _ := cmd.Flags().GetBool("volumes")
			force, _ := cmd.Flags().GetBool("force")
			data, err := vm.DialControl(cmd.Context(), "system-prune", nil, map[string]any{
				"all": all, "volumes": volumes, "force": force,
			})
			if err != nil {
				return fmt.Errorf("system prune failed: %w", err)
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}
	prune.Flags().BoolP("all", "a", false, "Remove all unreferenced images")
	prune.Flags().Bool("volumes", false, "Also prune unused volumes")
	prune.Flags().BoolP("force", "f", false, "Do not prompt for confirmation")

	cmd.AddCommand(df, eventsCmd, prune)
	return cmd
}
