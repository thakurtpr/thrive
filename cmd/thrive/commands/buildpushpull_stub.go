//go:build darwin

package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/image"
)

func BuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build -t [tag] [path]",
		Short: "Build from Thrivefile",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("build: requires Linux runtime — start the VM with `thrive desktop start`")
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
			return fmt.Errorf("push: requires Linux runtime — start the VM with `thrive desktop start`")
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "Registry username")
	cmd.Flags().StringVar(&password, "password", "", "Registry password")
	_ = username
	_ = password
	return cmd
}

// PullCmd pulls an OCI image from any registry to ~/.thrive/images/ on the
// macOS host. The VM reads images from the same store via virtiofs.
// No `thrive desktop start` needed — pull works without a running VM.
func PullCmd() *cobra.Command {
	var username, password string

	cmd := &cobra.Command{
		Use:   "pull [image]",
		Short: "Pull image from OCI registry (docker.io, ghcr.io, quay.io, etc.)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			fmt.Printf("Pulling %s ...\n", ref)

			img, err := image.Pull(context.Background(), ref, image.PullOptions{
				Username: username,
				Password: password,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			digest := img.Digest
			if len(digest) > 19 {
				digest = digest[:19]
			}
			fmt.Printf("Pulled: %s\n", img.Ref)
			fmt.Printf("Digest: %s\n", digest)
			fmt.Printf("Layers: %d\n", len(img.Layers))
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "Registry username")
	cmd.Flags().StringVar(&password, "password", "", "Registry password")
	return cmd
}
