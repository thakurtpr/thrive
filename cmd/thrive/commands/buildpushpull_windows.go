//go:build windows

package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/registry"
	"github.com/thakurprasadrout/thrive/internal/signing"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

// BuildCmd stays unimplemented on Windows: cmd/thrived's control-socket
// dispatch has no "build" handler on any platform yet (Thrivefile builds
// run many nested containers — bigger lift than a single bridge call).
func BuildCmd() *cobra.Command {
	return &cobra.Command{Use: "build", Short: "Build image (requires Linux; run inside WSL2)", RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("build requires Linux; run inside WSL2")
	}}
}

// PushCmd's flags exist so registry credentials have a consistent CLI
// surface across platforms, but thrived has no "push" handler yet on any
// platform (matches macOS's own PushCmd stub) — fail honestly rather than
// silently drop the upload.
func PushCmd() *cobra.Command {
	var username, password string
	var quiet bool
	cmd := &cobra.Command{
		Use:   "push [image]",
		Short: "Push image to registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("push is not yet implemented by the Thrive VM daemon")
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "Registry username")
	cmd.Flags().StringVar(&password, "password", "", "Registry password")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Suppress progress output")
	return cmd
}

// PullCmd pulls an OCI image from inside the Thrive VM via the control
// socket. Unlike macOS (which pulls on the host because Apple's VF NAT
// blocks the VM from reaching the internet), Windows VMs have normal
// internet access, so pulling inside the VM — the same place `thrive run`
// already syncs images to — is simpler and keeps one code path.
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

			opts := map[string]any{}
			if username == "" {
				if c := registry.StoredAuth(registry.RegistryHost(ref)); c != nil {
					username, password = c.Username, c.Password
				}
			}
			if username != "" {
				opts["username"] = username
			}
			if password != "" {
				opts["password"] = password
			}
			if platform != "" {
				opts["platform"] = platform
			}
			if allTags {
				opts["all_tags"] = true
			}
			if quiet {
				opts["quiet"] = true
			}

			data, err := vm.DialControl(cmd.Context(), "pull", []string{ref}, opts)
			if err != nil {
				return err
			}

			var result map[string]any
			json.Unmarshal(data, &result)

			// Cosign verification is host-side (remote registry lookup against
			// the explicit key file — no image bytes needed). On failure the
			// just-pulled image is removed from the VM so untrusted content
			// is never left behind. With --all-tags every tag is verified.
			if verifyPull {
				verifyRefs := []string{ref}
				if allTags {
					tags, err := registry.ListRepoTags(context.Background(), ref, username, password)
					if err != nil {
						return fmt.Errorf("verify: list tags for %s: %w", ref, err)
					}
					verifyRefs = tags
				}
				for _, r := range verifyRefs {
					if err := signing.VerifyCosignImage(context.Background(), r, verifyKey, username, password); err != nil {
						_, _ = vm.DialControl(cmd.Context(), "rmi", []string{r}, nil)
						return fmt.Errorf("cosign verification failed for %s: %w", r, err)
					}
					if !quiet {
						fmt.Printf("Verified: %s\n", r)
					}
				}
			}

			digest, _ := result["digest"].(string)
			layers := 0
			if lf, ok := result["layers"].(float64); ok {
				layers = int(lf)
			}
			if !quiet {
				fmt.Printf("Pulled: %s\n", ref)
				fmt.Printf("Digest: %s\n", digest)
				fmt.Printf("Layers: %d\n", layers)
			} else {
				fmt.Println(ref)
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
