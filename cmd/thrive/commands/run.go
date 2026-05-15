package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func RunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run [image] [command...]",
		Short: "Run a container",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("run command - stub")
			os.Exit(0)
		},
	}
}
