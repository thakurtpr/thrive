//go:build windows

package commands

import "github.com/spf13/cobra"

func DesktopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "desktop",
		Short: "Desktop VM (not supported on Windows — use WSL2 or Hyper-V)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("desktop VM is not available on Windows; use 'thrive desktop' inside WSL2")
			return nil
		},
	}
	return cmd
}
