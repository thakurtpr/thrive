//go:build !linux

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func StartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [container]",
		Short: "Start a stopped container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			checkpointName, _ := cmd.Flags().GetString("checkpoint")
			opts := map[string]any{}
			if checkpointName != "" {
				opts["checkpoint"] = checkpointName
			}
			if _, err := vm.DialControl(cmd.Context(), "start", []string{id}, opts); err != nil {
				return fmt.Errorf("start failed: %w", err)
			}
			fmt.Printf("container %s started\n", id)
			return nil
		},
	}
	cmd.Flags().String("checkpoint", "", "Restore from checkpoint (requires CRIU in the VM)")
	return cmd
}
