//go:build !linux

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func KillCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kill",
		Short: "Kill a running container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			containerID := args[0]
			_, err := vm.DialControl(cmd.Context(), "kill", []string{containerID}, nil)
			if err != nil {
				return fmt.Errorf("kill failed: %w", err)
			}
			fmt.Printf("container %s killed\n", containerID)
			return nil
		},
	}
}