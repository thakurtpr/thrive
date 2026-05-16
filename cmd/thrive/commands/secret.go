//go:build linux

package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/secrets"
	"github.com/thakurprasadrout/thrive/internal/telemetry"
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
			if err := telemetry.Init(); err != nil {
				fmt.Fprintf(os.Stderr, "telemetry init: %v\n", err)
				os.Exit(1)
			}

			id, err := secrets.Create(args[0], args[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating secret: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Secret %q created with ID %s\n", args[0], id)
		},
	}
}

func secretLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List secrets",
		Run: func(cmd *cobra.Command, args []string) {
			if err := telemetry.Init(); err != nil {
				fmt.Fprintf(os.Stderr, "telemetry init: %v\n", err)
				os.Exit(1)
			}

			names, err := secrets.List()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error listing secrets: %v\n", err)
				os.Exit(1)
			}
			if len(names) == 0 {
				fmt.Println("No secrets found.")
				return
			}
			for _, n := range names {
				fmt.Println(n)
			}
		},
	}
}

func secretRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm [name]",
		Short: "Remove a secret",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := telemetry.Init(); err != nil {
				fmt.Fprintf(os.Stderr, "telemetry init: %v\n", err)
				os.Exit(1)
			}

			if err := secrets.Delete(args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error removing secret: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Secret %q removed.\n", args[0])
		},
	}
}