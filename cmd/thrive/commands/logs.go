package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func LogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs [container]",
		Short: "View container logs",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("logs command - stub")
		},
	}
}
