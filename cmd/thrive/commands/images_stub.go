//go:build darwin

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
		Short: "List pulled images",
		RunE: func(cmd *cobra.Command, args []string) error {
			imgs, err := image.List(context.Background())
			if err != nil {
				return err
			}
			rows := make([]imageRow, 0, len(imgs))
			for _, img := range imgs {
				rows = append(rows, imageRow{Ref: img.Ref, Digest: img.Digest, Layers: len(img.Layers)})
			}
			formatImagesTable(os.Stdout, rows)
			return nil
		},
	}
}

func RmiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rmi [image]",
		Short: "Remove a pulled image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := image.Remove(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Printf("Image %s removed\n", args[0])
			return nil
		},
	}
}
