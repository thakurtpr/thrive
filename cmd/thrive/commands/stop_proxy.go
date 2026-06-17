//go:build !linux

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func StopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop [container]",
		Short: "Stop a running container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if _, err := vm.DialControl(cmd.Context(), "stop", []string{id}, nil); err != nil {
				return fmt.Errorf("stop failed: %w", err)
			}
			fmt.Printf("container %s stopped\n", id)
			return nil
		},
	}
}
