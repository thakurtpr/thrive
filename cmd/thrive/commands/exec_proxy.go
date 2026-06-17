//go:build !linux

package commands

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func ExecCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "exec [container] [command...]",
		Short: "Execute a command in a running container",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			containerID := args[0]
			command := args[1:]
			return vm.DialControlStream(cmd.Context(), "exec", append([]string{containerID}, command...), nil, os.Stdout)
		},
	}
}
