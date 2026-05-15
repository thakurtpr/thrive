package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func KillCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kill [container]",
		Short: "Kill a container",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("kill command - stub")
		},
	}
}
