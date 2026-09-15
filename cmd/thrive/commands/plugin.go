//go:build linux

package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/plugin"
)

// PluginCmd manages engine plugins (management plane).
func PluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage engine plugins",
	}

	install := &cobra.Command{
		Use: "install [name] [source]", Short: "Install a plugin from a directory or tarball", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := plugin.Install(args[0], args[1])
			if err != nil {
				return err
			}
			fmt.Printf("Installed plugin %s (disabled — run `thrive plugin enable %s`)\n", p.Name, p.Name)
			return nil
		},
	}

	enable := &cobra.Command{
		Use: "enable [plugin]", Short: "Enable a plugin", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return plugin.Enable(args[0])
		},
	}

	disable := &cobra.Command{
		Use: "disable [plugin]", Short: "Disable a plugin", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return plugin.Disable(args[0])
		},
	}

	inspectP := &cobra.Command{
		Use: "inspect [plugin]", Short: "Inspect a plugin", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := plugin.Inspect(args[0])
			if err != nil {
				return err
			}
			out, _ := json.MarshalIndent(p, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	ls := &cobra.Command{
		Use: "ls", Short: "List plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			plugins, err := plugin.List()
			if err != nil {
				return err
			}
			fmt.Printf("%-30s %-8s %s\n", "NAME", "ENABLED", "CREATED")
			for _, p := range plugins {
				fmt.Printf("%-30s %-8v %s\n", p.Name, p.Enabled, p.Created.Format("2006-01-02 15:04"))
			}
			return nil
		},
	}

	rm := &cobra.Command{
		Use: "rm [plugin]", Short: "Remove a plugin", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			return plugin.Remove(args[0], force)
		},
	}
	rm.Flags().BoolP("force", "f", false, "Remove an enabled plugin")

	cmd.AddCommand(install, enable, disable, inspectP, ls, rm)
	return cmd
}
