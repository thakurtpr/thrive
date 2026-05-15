package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func BuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build -t [tag] [path]",
		Short: "Build from Thrivefile",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("build command - stub")
		},
	}
}

func PushCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "push [image]",
		Short: "Push image to registry",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("push command - stub")
		},
	}
}

func PullCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pull [image]",
		Short: "Pull image from registry",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("pull command - stub")
		},
	}
}
