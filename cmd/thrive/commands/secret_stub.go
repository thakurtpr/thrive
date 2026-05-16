//go:build !linux

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func SecretCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret",
		Short: "Manage secrets",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("secret: requires Linux — run inside the Thrive VM or on a Linux host")
		},
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "create [name] [value]",
			Short: "Create a secret",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				return fmt.Errorf("secret create: requires Linux — run inside the Thrive VM")
			},
		},
		&cobra.Command{
			Use:   "ls",
			Short: "List secrets",
			RunE: func(cmd *cobra.Command, args []string) error {
				return fmt.Errorf("secret ls: requires Linux — run inside the Thrive VM")
			},
		},
		&cobra.Command{
			Use:   "rm [name]",
			Short: "Remove a secret",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return fmt.Errorf("secret rm: requires Linux — run inside the Thrive VM")
			},
		},
	)
	return cmd
}
