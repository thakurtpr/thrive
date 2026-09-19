//go:build !linux

package commands

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func ExecCmd() *cobra.Command {
	var workdir string
	var interactive, tty bool
	cmd := &cobra.Command{
		Use:   "exec [container] [command...]",
		Short: "Execute a command in a running container",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			containerID := args[0]
			command := args[1:]
			opts := map[string]any{}
			if envVars, _ := cmd.Flags().GetStringArray("env"); len(envVars) > 0 {
				opts["env"] = envVars
			}
			if workdir, _ := cmd.Flags().GetString("workdir"); workdir != "" {
				opts["workdir"] = workdir
			}
			return vm.DialControlStream(cmd.Context(), "exec", append([]string{containerID}, command...), opts, os.Stdout)
		},
	}
	cmd.Flags().StringArrayP("env", "e", nil, "Set environment variables")
	cmd.Flags().StringVarP(&workdir, "workdir", "w", "", "Working directory inside the container")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Keep stdin open (implied: stdin is attached)")
	cmd.Flags().BoolVarP(&tty, "tty", "t", false, "Allocate a pseudo-TTY (informational: output is not raw-PTY)")
	cmd.Flags().SetInterspersed(false)
	return cmd
}
