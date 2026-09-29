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
			rows := make([]imageRow, 0, len(images))
			for _, img := range images {
				rows = append(rows, imageRow{Ref: img.Ref, Digest: img.Digest, Layers: len(img.Layers)})
			}
			formatImagesTable(os.Stdout, rows)
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
