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
		commands.CreateCmd(),
		commands.PauseCmd(),
		commands.UnpauseCmd(),
		commands.WaitCmd(),
		commands.RenameCmd(),
		commands.StatsCmd(),
		commands.UpdateCmd(),
		commands.TopCmd(),
		commands.PortCmd(),
		commands.DiffCmd(),
		commands.ExportCmd(),
		commands.CommitCmd(),
		commands.SaveCmd(),
		commands.LoadCmd(),
		commands.ImportCmd(),
		commands.HistoryCmd(),
		commands.LoginCmd(),
		commands.LogoutCmd(),
		commands.SearchCmd(),
		commands.ManifestCmd(),
		commands.NetworkCmd(),
		commands.VolumeCmd(),
		commands.SwarmCmd(),
		commands.ServiceCmd(),
		commands.StackCmd(),
		commands.BuildxCmd(),
		commands.ContextCmd(),
		commands.PluginCmd(),
		commands.CheckpointCmd(),
		commands.AttachCmd(),
		commands.VersionCmd(),
		commands.NodeCmd(),
		commands.ConfigCmd(),
		commands.BuilderCmd(),
		commands.ImageCmd(),
		commands.ContainerCmd(),	)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
