package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func ImagesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "images",
		Short: "List images",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("images command - stub")
		},
	}
}

func rmiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rmi [image]",
		Short: "Remove an image",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("rmi command - stub")
		},
	}
}
