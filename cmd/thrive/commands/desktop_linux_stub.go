//go:build linux

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// DesktopCmd is a no-op on Linux — containers run natively without a VM.
func DesktopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "desktop",
		Short: "Manage Thrive Desktop VM (not needed on Linux)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("desktop: Linux runs containers natively — no VM needed")
		},
	}
}
