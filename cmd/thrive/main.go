//go:build linux || darwin || windows

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/cmd/thrive/commands"
)

func main() {
	root := &cobra.Command{
		Use:   "thrive",
		Short: "THRIVE — THakur Runtime Isolation Virtualization Engine",
	}
	root.AddCommand(
		commands.RunCmd(),
		commands.PsCmd(),
		commands.KillCmd(),
		commands.RmCmd(),
		commands.LogsCmd(),
		commands.ImagesCmd(),
		commands.BuildCmd(),
		commands.PushCmd(),
		commands.PullCmd(),
		commands.SecretCmd(),
		commands.MetricsCmd(),
		commands.SystemCmd(),
	)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
