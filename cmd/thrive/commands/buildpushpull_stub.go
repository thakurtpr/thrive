//go:build !linux

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func BuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build -t [tag] [path]",
		Short: "Build from Thrivefile",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("build: requires Linux — run inside the Thrive VM or on a Linux host")
		},
	}
}

func PushCmd() *cobra.Command {
	var username, password string
	cmd := &cobra.Command{
		Use:   "push [image]",
		Short: "Push image to registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("push: requires Linux — run inside the Thrive VM or on a Linux host")
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "Registry username")
	cmd.Flags().StringVar(&password, "password", "", "Registry password")
	return cmd
}

func PullCmd() *cobra.Command {
	var username, password string
	cmd := &cobra.Command{
		Use:   "pull [image]",
		Short: "Pull image from registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("pull: requires Linux — run inside the Thrive VM or on a Linux host")
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "Registry username")
	cmd.Flags().StringVar(&password, "password", "", "Registry password")
	return cmd
}
