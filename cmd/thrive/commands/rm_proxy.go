//go:build !linux

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func RmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm",
		Short: "Remove a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			containerID := args[0]
			force, _ := cmd.Flags().GetBool("force")
			_, err := vm.DialControl(cmd.Context(), "rm", []string{containerID}, map[string]any{"force": force})
			if err != nil {
				return fmt.Errorf("rm failed: %w", err)
			}
			fmt.Printf("container %s removed\n", containerID)
			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Remove a running container (kills first)")
	return cmd
}
