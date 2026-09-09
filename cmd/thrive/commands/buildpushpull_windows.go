//go:build windows

package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

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
	return cmd
}

// PullCmd pulls an OCI image from inside the Thrive VM via the control
// socket. Unlike macOS (which pulls on the host because Apple's VF NAT
// blocks the VM from reaching the internet), Windows VMs have normal
// internet access, so pulling inside the VM — the same place `thrive run`
// already syncs images to — is simpler and keeps one code path.
func PullCmd() *cobra.Command {
	var username, password string
	cmd := &cobra.Command{
		Use:   "pull [image]",
		Short: "Pull image from OCI registry (docker.io, ghcr.io, quay.io, etc.)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			fmt.Printf("Pulling %s ...\n", ref)

			opts := map[string]any{}
			if username != "" {
				opts["username"] = username
			}
			if password != "" {
				opts["password"] = password
			}

			data, err := vm.DialControl(cmd.Context(), "pull", []string{ref}, opts)
			if err != nil {
				return err
			}

			var result map[string]any
			json.Unmarshal(data, &result)

			digest, _ := result["digest"].(string)
			layers := 0
			if lf, ok := result["layers"].(float64); ok {
				layers = int(lf)
			}
			fmt.Printf("Pulled: %s\n", ref)
			fmt.Printf("Digest: %s\n", digest)
			fmt.Printf("Layers: %d\n", layers)
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "Registry username")
	cmd.Flags().StringVar(&password, "password", "", "Registry password")
	return cmd
}
