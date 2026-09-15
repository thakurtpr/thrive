//go:build !linux

package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func InspectCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "inspect [container|image]",
		Short: "Display detailed information about a container or image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			data, err := vm.DialControl(cmd.Context(), "inspect", []string{id}, nil)
			if err != nil {
				return fmt.Errorf("inspect failed: %w", err)
			}
			var result map[string]any
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("inspect: parse response: %w", err)
			}
			if format != "" {
				out, err := renderFormat(format, result)
				if err != nil {
					return err
				}
				fmt.Println(out)
				return nil
			}
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}
	cmd.Flags().String("format", "", "Format output with a Go template")
	return cmd
}
