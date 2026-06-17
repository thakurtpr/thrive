//go:build !linux

package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func InspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect [container]",
		Short: "Display detailed information about a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			data, err := vm.DialControl(cmd.Context(), "inspect", []string{id}, nil)
			if err != nil {
				return fmt.Errorf("inspect failed: %w", err)
			}
			var result any
			json.Unmarshal(data, &result)
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}
}
