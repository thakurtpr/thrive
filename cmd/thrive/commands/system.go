//go:build linux

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func SystemCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "System operations",
	}
	cmd.AddCommand(systemInfoCmd(), systemCleanCmd())
	return cmd
}

func systemInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show system info",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("system info - stub")
		},
	}
}

func systemCleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Clean up unused resources",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("system clean - stub")
		},
	}
}
