//go:build darwin

package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/registry"
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
	var quiet bool

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
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Suppress progress output")
	_ = username
	_ = password
	_ = quiet
	return cmd
}

// PullCmd pulls an OCI image from any registry to ~/.thrive/images/ on the
// macOS host. The VM reads images from the same store via virtiofs.
// No `thrive desktop start` needed — pull works without a running VM.
func PullCmd() *cobra.Command {
	var username, password, platform string
	var quiet, allTags, verifyPull bool
	var verifyKey string

	cmd := &cobra.Command{
		Use:   "pull [image]",
		Short: "Pull image from OCI registry (docker.io, ghcr.io, quay.io, etc.)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			if verifyPull && verifyKey == "" {
				return fmt.Errorf("--verify requires --verify-key <cosign.pub>")
			}
			if !quiet {
				fmt.Printf("Pulling %s ...\n", ref)
			}

			if username == "" {
				if c := registry.StoredAuth(registry.RegistryHost(ref)); c != nil {
					username, password = c.Username, c.Password
				}
			}
			opts := image.PullOptions{Username: username, Password: password, Platform: platform}
			refs := []string{ref}
			if allTags {
				var err error
				if refs, err = listRepoTags(ref, username, password); err != nil {
					return err
				}
			}
			for _, r := range refs {
				img, err := image.Pull(context.Background(), r, opts)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}

				if !verifyPulledImage(context.Background(), r, img.Ref, img.Digest, username, password, verifyPull, verifyKey, quiet) {
					os.Exit(1)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "Registry username")
	cmd.Flags().StringVar(&password, "password", "", "Registry password")
	cmd.Flags().StringVar(&platform, "platform", "", "Platform (os/arch, e.g. linux/arm64)")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Suppress progress output")
	cmd.Flags().BoolVarP(&allTags, "all-tags", "a", false, "Pull all tagged images in the repository")
	cmd.Flags().BoolVar(&verifyPull, "verify", false, "Verify cosign signature after pull (requires --verify-key)")
	cmd.Flags().StringVar(&verifyKey, "verify-key", "", "PEM public key file for cosign verification")
	return cmd
}
