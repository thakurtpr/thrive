package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func SecretCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret",
		Short: "Manage secrets",
	}
	cmd.AddCommand(secretCreateCmd(), secretLsCmd(), secretRmCmd())
	return cmd
}

func secretCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create [name] [value]",
		Short: "Create a secret",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("secret create - stub")
		},
	}
}

func secretLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List secrets",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("secret ls - stub")
		},
	}
}

func secretRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm [name]",
		Short: "Remove a secret",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("secret rm - stub")
		},
	}
}
