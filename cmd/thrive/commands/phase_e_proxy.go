//go:build !linux

package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/buildx"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

// BuildxCmd proxies extended builds to the VM daemon. Local build
// execution needs the Linux runtime; builder records are local.
func BuildxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "buildx",
		Short: "Extended builds (multi-platform records, bake, builders, cache)",
	}

	buildCmd := &cobra.Command{
		Use: "build [path]", Short: "Build from Thrivefile or Dockerfile",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("buildx build: requires Linux runtime — build contexts are not synced to the VM")
		},
	}
	// Registered for CLI parity (scripts passing them get the honest
	// Linux-required error instead of `unknown flag`), matching the R2
	// logs --since/--until/--timestamps pattern.
	buildCmd.Flags().String("platform", "", "Target platform (native only)")
	buildCmd.Flags().StringP("file", "f", "", "Build definition file")
	buildCmd.Flags().StringP("tag", "t", "", "Image tag")
	buildCmd.Flags().Bool("no-cache", false, "Do not use the build cache")

	bake := &cobra.Command{
		Use: "bake [service...]", Short: "Build all services from a compose file",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("buildx bake: requires Linux runtime — build contexts are not synced to the VM")
		},
	}
	bake.Flags().StringP("file", "f", "docker-compose.yml", "Compose file path")

	ls := &cobra.Command{
		Use: "ls", Short: "List builders",
		RunE: func(cmd *cobra.Command, args []string) error {
			builders, err := buildx.ListBuilders()
			if err != nil {
				return err
			}
			fmt.Printf("%-20s %-16s %s\n", "NAME", "DRIVER", "CURRENT")
			for _, b := range builders {
				cur := ""
				if b.Current {
					cur = "*"
				}
				fmt.Printf("%-20s %-16s %s\n", b.Name, b.Driver, cur)
			}
			return nil
		},
	}

	create := &cobra.Command{
		Use: "create [name]", Short: "Create a builder", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			driver, _ := cmd.Flags().GetString("driver")
			b, err := buildx.CreateBuilder(args[0], driver, "")
			if err != nil {
				return err
			}
			fmt.Printf("Created builder %s (%s)\n", b.Name, b.Driver)
			return nil
		},
	}
	create.Flags().String("driver", "thrive", "Builder driver")

	rm := &cobra.Command{
		Use: "rm [name]", Short: "Remove a builder", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return buildx.RemoveBuilder(args[0])
		},
	}

	inspectB := &cobra.Command{
		Use: "inspect [name]", Short: "Inspect a builder", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := buildx.InspectBuilder(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Name: %s\nDriver: %s\nCurrent: %v\n", b.Name, b.Driver, b.Current)
			return nil
		},
	}

	use := &cobra.Command{
		Use: "use [name]", Short: "Set the current builder", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return buildx.UseBuilder(args[0])
		},
	}

	du := &cobra.Command{
		Use: "du", Short: "Show build cache usage",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "buildx-du", nil, nil)
			if err != nil {
				return fmt.Errorf("buildx du failed: %w", err)
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			fmt.Printf("Build cache: %v bytes\n", result["size"])
			return nil
		},
	}

	prune := &cobra.Command{
		Use: "prune", Short: "Clear the build cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "buildx-prune", nil, nil)
			if err != nil {
				return fmt.Errorf("buildx prune failed: %w", err)
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			fmt.Printf("Reclaimed %v bytes\n", result["reclaimed"])
			return nil
		},
	}

	cmd.AddCommand(buildCmd, bake, ls, create, rm, inspectB, use, du, prune)
	return cmd
}

// PluginCmd proxies plugin management to the VM daemon.
func PluginCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "plugin", Short: "Manage engine plugins"}

	simple := func(use, short, bridge string, n int) *cobra.Command {
		return &cobra.Command{
			Use: use, Short: short, Args: cobra.ExactArgs(n),
			RunE: func(cmd *cobra.Command, args []string) error {
				force, _ := cmd.Flags().GetBool("force")
				opts := map[string]any{}
				if force {
					opts["force"] = true
				}
				data, err := vm.DialControl(cmd.Context(), bridge, args, opts)
				if err != nil {
					return fmt.Errorf("%s failed: %w", bridge, err)
				}
				var result any
				json.Unmarshal(data, &result)
				out, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(out))
				return nil
			},
		}
	}

	install := simple("install [name] [source]", "Install a plugin (source must exist in the VM)", "plugin-install", 2)
	enable := simple("enable [plugin]", "Enable a plugin", "plugin-enable", 1)
	disable := simple("disable [plugin]", "Disable a plugin", "plugin-disable", 1)
	inspectP := simple("inspect [plugin]", "Inspect a plugin", "plugin-inspect", 1)
	ls := simple("ls", "List plugins", "plugin-ls", 0)
	rm := simple("rm [plugin]", "Remove a plugin", "plugin-rm", 1)
	rm.Flags().BoolP("force", "f", false, "Remove an enabled plugin")

	cmd.AddCommand(install, enable, disable, inspectP, ls, rm)
	return cmd
}

// CheckpointCmd proxies checkpoints to the VM daemon (needs CRIU there).
func CheckpointCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "checkpoint", Short: "Manage container checkpoints (requires CRIU in the VM)"}

	create := &cobra.Command{
		Use: "create [container] [name]", Short: "Checkpoint a running container", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := vm.DialControl(cmd.Context(), "checkpoint-create", args, nil)
			return err
		},
	}
	ls := &cobra.Command{
		Use: "ls [container]", Short: "List checkpoints", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "checkpoint-ls", args, nil)
			if err != nil {
				return err
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			if cps, ok := result["checkpoints"].([]any); ok {
				fmt.Printf("%-20s %s\n", "NAME", "CREATED")
				for _, c := range cps {
					cm, _ := c.(map[string]any)
					fmt.Printf("%-20v %v\n", cm["name"], cm["created"])
				}
			}
			return nil
		},
	}
	rm := &cobra.Command{
		Use: "rm [container] [name]", Short: "Remove a checkpoint", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := vm.DialControl(cmd.Context(), "checkpoint-rm", args, nil)
			return err
		},
	}

	cmd.AddCommand(create, ls, rm)
	return cmd
}
