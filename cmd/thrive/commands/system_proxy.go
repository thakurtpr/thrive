//go:build !linux

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func SystemCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "system",
		Short: "Show Thrive system information",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "system", nil, nil)
			if err != nil {
				return fmt.Errorf("system info failed: %w", err)
			}
			fmt.Print(string(data))
			return nil
		},
	}
}