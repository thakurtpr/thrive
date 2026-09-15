//go:build !linux

package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

// VolumeCmd proxies volume management to the VM daemon.
func VolumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "volume",
		Short: "Manage named volumes",
	}

	create := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a volume",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "volume-create", args, nil)
			if err != nil {
				return fmt.Errorf("volume create failed: %w", err)
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			fmt.Printf("%v\n", result["name"])
			return nil
		},
	}

	ls := &cobra.Command{
		Use:   "ls",
		Short: "List volumes",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "volume-ls", nil, nil)
			if err != nil {
				return fmt.Errorf("volume ls failed: %w", err)
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			if vols, ok := result["volumes"].([]any); ok {
				fmt.Printf("%-25s %s\n", "NAME", "PATH")
				for _, v := range vols {
					item, _ := v.(map[string]any)
					fmt.Printf("%-25v %v\n", item["name"], item["path"])
				}
			}
			return nil
		},
	}

	inspect := &cobra.Command{
		Use:   "inspect [volume]",
		Short: "Display detailed information about a volume",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "volume-inspect", args, nil)
			if err != nil {
				return fmt.Errorf("volume inspect failed: %w", err)
			}
			var result any
			json.Unmarshal(data, &result)
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	rm := &cobra.Command{
		Use:   "rm [volume...]",
		Short: "Remove volumes",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			_, err := vm.DialControl(cmd.Context(), "volume-rm", args, map[string]any{"force": force})
			if err != nil {
				return fmt.Errorf("volume rm failed: %w", err)
			}
			for _, name := range args {
				fmt.Printf("Removed volume %s\n", name)
			}
			return nil
		},
	}
	rm.Flags().BoolP("force", "f", false, "Remove despite being in use")

	prune := &cobra.Command{
		Use:   "prune",
		Short: "Remove all unused volumes",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "volume-prune", nil, nil)
			if err != nil {
				return fmt.Errorf("volume prune failed: %w", err)
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			fmt.Printf("Pruned %v volume(s)\n", result["removed"])
			return nil
		},
	}

	cmd.AddCommand(create, ls, inspect, rm, prune)
	return cmd
}
