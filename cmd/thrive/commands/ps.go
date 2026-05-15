package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func PsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ps",
		Short: "List containers",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("ps command - stub")
		},
	}
}
