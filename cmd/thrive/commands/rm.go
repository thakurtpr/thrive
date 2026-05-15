package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func RmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm [container]",
		Short: "Remove a container",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("rm command - stub")
		},
	}
}
