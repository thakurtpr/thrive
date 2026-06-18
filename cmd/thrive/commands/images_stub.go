//go:build !linux

package commands

import (
	"context"
	"fmt"

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

			if len(imgs) == 0 {
				fmt.Println("no images — run `thrive pull <image>` first")
				return nil
			}

			fmt.Printf("%-50s %-22s %s\n", "REPOSITORY", "DIGEST", "LAYERS")
			fmt.Println("────────────────────────────────────────────────────────────────────────────────")
			for _, img := range imgs {
				digest := img.Digest
				if len(digest) > 19 {
					digest = digest[:19]
				}
				fmt.Printf("%-50s %-22s %d\n", img.Ref, digest, len(img.Layers))
			}
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
