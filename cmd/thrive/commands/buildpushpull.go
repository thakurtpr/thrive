package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/pkg/build"
)

var buildTag string

func BuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build -t [tag] [path]",
		Short: "Build from Thrivefile",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			path := args[0]
			if len(args) > 1 {
				buildTag = args[1]
			}

			graph, err := build.ParseThrivefile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing Thrivefile: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Building %s from %s\n", graph.BaseImage, path)

			result, err := build.Execute(ctx, graph, build.BuildOptions{
				Tag:       buildTag,
				NoCache:   false,
				ContextDir: path,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error building: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Build complete: %s (%d steps)\n", result.ImageID, result.Steps)
		},
	}
	cmd.Flags().StringVarP(&buildTag, "tag", "t", "", "Image tag")
	return cmd
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
