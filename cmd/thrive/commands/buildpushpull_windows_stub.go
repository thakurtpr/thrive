//go:build windows

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func BuildCmd() *cobra.Command {
	return &cobra.Command{Use: "build", Short: "Build image (requires Linux; run inside WSL2)", RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("build requires Linux; run inside WSL2")
	}}
}

func PushCmd() *cobra.Command {
	return &cobra.Command{Use: "push", Short: "Push image (requires Linux; run inside WSL2)", RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("push requires Linux; run inside WSL2")
	}}
}

func PullCmd() *cobra.Command {
	return &cobra.Command{Use: "pull", Short: "Pull image (requires Linux; run inside WSL2)", RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("pull requires Linux; run inside WSL2")
	}}
}
