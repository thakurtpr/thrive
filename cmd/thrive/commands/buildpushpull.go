//go:build linux

package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/image"
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
	var username, password string

	cmd := &cobra.Command{
		Use:   "push [image]",
		Short: "Push image to registry",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			ref := args[0]
			fmt.Printf("Pushing %s ...\n", ref)
			if err := image.Push(ctx, ref, image.PushOptions{
				Username: username,
				Password: password,
			}); err != nil {
				fmt.Fprintf(os.Stderr, "Error pushing image: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Pushed: %s\n", ref)
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
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			ref := args[0]
			fmt.Printf("Pulling %s ...\n", ref)
			img, err := image.Pull(ctx, ref, image.PullOptions{
				Username: username,
				Password: password,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error pulling image: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Pulled: %s@%s\n", img.Ref, img.Digest[:12])
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "Registry username")
	cmd.Flags().StringVar(&password, "password", "", "Registry password")
	return cmd
}
