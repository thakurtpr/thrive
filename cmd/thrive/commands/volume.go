//go:build linux

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/runtime"
	"github.com/thakurprasadrout/thrive/internal/volume"
)

// VolumeCmd manages named volumes (docker volume parity).
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
			v, err := volume.Create(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("%s (%s)\n", v.Name, v.Path)
			return nil
		},
	}

	ls := &cobra.Command{
		Use:   "ls",
		Short: "List volumes",
		RunE: func(cmd *cobra.Command, args []string) error {
			vols, err := volume.List()
			if err != nil {
				return err
			}
			fmt.Printf("%-25s %s\n", "NAME", "PATH")
			for _, v := range vols {
				fmt.Printf("%-25s %s\n", v.Name, v.Path)
			}
			return nil
		},
	}

	inspect := &cobra.Command{
		Use:   "inspect [volume]",
		Short: "Display detailed information about a volume",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := volume.Inspect(args[0])
			if err != nil {
				return err
			}
			out, _ := json.MarshalIndent(v, "", "  ")
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
			inUse := volumeInUseChecker(context.Background())
			failed := false
			for _, name := range args {
				if err := volume.Remove(name, force, inUse); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					failed = true
					continue
				}
				fmt.Printf("Removed volume %s\n", name)
			}
			if failed {
				return fmt.Errorf("one or more volumes could not be removed")
			}
			return nil
		},
	}
	rm.Flags().BoolP("force", "f", false, "Remove despite being in use")

	prune := &cobra.Command{
		Use:   "prune",
		Short: "Remove all unused volumes",
		RunE: func(cmd *cobra.Command, args []string) error {
			removed, err := volume.Prune(volumeInUseChecker(context.Background()))
			if err != nil {
				return err
			}
			for _, name := range removed {
				fmt.Printf("Removed volume %s\n", name)
			}
			fmt.Printf("Pruned %d volume(s)\n", len(removed))
			return nil
		},
	}

	cmd.AddCommand(create, ls, inspect, rm, prune)
	return cmd
}

// volumeInUseChecker scans container configs for mounts using a volume path.
func volumeInUseChecker(ctx context.Context) func(path string) bool {
	return func(path string) bool {
		entries, err := os.ReadDir("/run/thrive/containers")
		if err != nil {
			return false
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			data, err := os.ReadFile("/run/thrive/containers/" + e.Name() + "/config.json")
			if err != nil {
				continue
			}
			var cfg runtime.ContainerConfig
			if err := json.Unmarshal(data, &cfg); err != nil {
				continue
			}
			for _, m := range cfg.Mounts {
				if m.Source == path {
					return true
				}
			}
		}
		return false
	}
}
