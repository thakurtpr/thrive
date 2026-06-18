//go:build linux || darwin || windows

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/cmd/thrive/commands"
)

var (
	Version = "dev"
	Commit  = "unknown"
)

func main() {
	root := &cobra.Command{
		Use:   "thrive",
		Short: "THRIVE — THakur Runtime Isolation Virtualization Engine",
	}
	root.Version = Version + " (" + Commit + ")"
	root.SetVersionTemplate("thrive {{.Version}}\n")
	root.AddCommand(
		commands.RunCmd(),
		commands.ExecCmd(),
		commands.PsCmd(),
		commands.KillCmd(),
		commands.StopCmd(),
		commands.StartCmd(),
		commands.RestartCmd(),
		commands.RmCmd(),
		commands.LogsCmd(),
		commands.ImagesCmd(),
		commands.RmiCmd(),
		commands.InspectCmd(),
		commands.BuildCmd(),
		commands.PushCmd(),
		commands.PullCmd(),
		commands.ComposeCmd(),
		commands.SecretCmd(),
		commands.MetricsCmd(),
		commands.SystemCmd(),
		commands.TagCmd(),
		commands.CpCmd(),
		commands.SignCmd(),
		commands.VerifyCmd(),
		commands.DesktopCmd(),
	)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
