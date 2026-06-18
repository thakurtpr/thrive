//go:build windows

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func ImagesCmd() *cobra.Command {
	return &cobra.Command{Use: "images", Short: "List images (requires Linux; run inside WSL2)", RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("images requires Linux; run inside WSL2")
	}}
}

func RmiCmd() *cobra.Command {
	return &cobra.Command{Use: "rmi", Short: "Remove image (requires Linux; run inside WSL2)", RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("rmi requires Linux; run inside WSL2")
	}}
}
