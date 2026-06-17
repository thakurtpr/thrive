//go:build !linux

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func StartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start [container]",
		Short: "Start a stopped container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if _, err := vm.DialControl(cmd.Context(), "start", []string{id}, nil); err != nil {
				return fmt.Errorf("start failed: %w", err)
			}
			fmt.Printf("container %s started\n", id)
			return nil
		},
	}
}
