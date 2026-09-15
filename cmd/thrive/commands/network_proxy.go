//go:build !linux

package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

// NetworkCmd proxies network management to the VM daemon.
func NetworkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Manage container networks",
	}

	create := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			driver, _ := cmd.Flags().GetString("driver")
			subnet, _ := cmd.Flags().GetString("subnet")
			data, err := vm.DialControl(cmd.Context(), "network-create", args, map[string]any{
				"driver": driver, "subnet": subnet,
			})
			if err != nil {
				return fmt.Errorf("network create failed: %w", err)
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			fmt.Printf("%v (%v)\n", result["name"], result["subnet"])
			return nil
		},
	}
	create.Flags().String("driver", "bridge", "Network driver (only bridge)")
	create.Flags().String("subnet", "", "Subnet in CIDR format (auto-allocated if empty)")

	ls := &cobra.Command{
		Use:   "ls",
		Short: "List networks",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "network-ls", nil, nil)
			if err != nil {
				return fmt.Errorf("network ls failed: %w", err)
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			if nets, ok := result["networks"].([]any); ok {
				fmt.Printf("%-15s %-8s %-18s %s\n", "NAME", "DRIVER", "SUBNET", "CONTAINERS")
				for _, n := range nets {
					nm, _ := n.(map[string]any)
					fmt.Printf("%-15v %-8v %-18v %v\n", nm["name"], nm["driver"], nm["subnet"], nm["containers"])
				}
			}
			return nil
		},
	}

	inspect := &cobra.Command{
		Use:   "inspect [network]",
		Short: "Display detailed information about a network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "network-inspect", args, nil)
			if err != nil {
				return fmt.Errorf("network inspect failed: %w", err)
			}
			var result any
			json.Unmarshal(data, &result)
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	rm := &cobra.Command{
		Use:   "rm [network...]",
		Short: "Remove networks",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			_, err := vm.DialControl(cmd.Context(), "network-rm", args, map[string]any{"force": force})
			if err != nil {
				return fmt.Errorf("network rm failed: %w", err)
			}
			for _, name := range args {
				fmt.Printf("Removed network %s\n", name)
			}
			return nil
		},
	}
	rm.Flags().BoolP("force", "f", false, "Remove despite attached containers")

	connect := &cobra.Command{
		Use:   "connect [network] [container]",
		Short: "Connect a container to a network",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := vm.DialControl(cmd.Context(), "network-connect", args, nil); err != nil {
				return fmt.Errorf("network connect failed: %w", err)
			}
			fmt.Printf("Connected %s to %s\n", args[1], args[0])
			return nil
		},
	}

	disconnect := &cobra.Command{
		Use:   "disconnect [network] [container]",
		Short: "Disconnect a container from a network",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			_, err := vm.DialControl(cmd.Context(), "network-disconnect", args, map[string]any{"force": force})
			if err != nil {
				return fmt.Errorf("network disconnect failed: %w", err)
			}
			fmt.Printf("Disconnected %s from %s\n", args[1], args[0])
			return nil
		},
	}
	disconnect.Flags().BoolP("force", "f", false, "Force disconnect")

	prune := &cobra.Command{
		Use:   "prune",
		Short: "Remove all unused networks",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "network-prune", nil, nil)
			if err != nil {
				return fmt.Errorf("network prune failed: %w", err)
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			fmt.Printf("Pruned %v network(s)\n", result["removed"])
			return nil
		},
	}

	cmd.AddCommand(create, ls, inspect, rm, connect, disconnect, prune)
	return cmd
}
