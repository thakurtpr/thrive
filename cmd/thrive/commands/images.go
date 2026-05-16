//go:build linux

package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/image"
)

func ImagesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "images",
		Short: "List images",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			images, err := image.List(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error listing images: %v\n", err)
				os.Exit(1)
			}
			for _, img := range images {
				fmt.Printf("%s\t%s\t%d layers\n", img.Ref, img.Digest[:12], len(img.Layers))
			}
		},
	}
}

func RmiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rmi [image]",
		Short: "Remove an image",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			imageRef := args[0]

			if err := image.Remove(ctx, imageRef); err != nil {
				fmt.Fprintf(os.Stderr, "Error removing image: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Image %s removed\n", imageRef)
		},
	}
}
