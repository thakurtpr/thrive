//go:build !linux

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func RestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart [container]",
		Short: "Restart a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if _, err := vm.DialControl(cmd.Context(), "restart", []string{id}, nil); err != nil {
				return fmt.Errorf("restart failed: %w", err)
			}
			fmt.Printf("container %s restarted\n", id)
			return nil
		},
	}
}
