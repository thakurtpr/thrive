//go:build !linux

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func ImagesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "images",
		Short: "List images",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("images: not yet implemented on this platform (run `thrive desktop start` first)")
		},
	}
}

func RmiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rmi [image]",
		Short: "Remove an image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("rmi: not yet implemented on this platform")
		},
	}
}
