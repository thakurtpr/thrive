package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/registry"
	"github.com/thakurprasadrout/thrive/internal/signing"
)

// listRepoTags returns fully-qualified refs for every tag in ref's
// repository (docker pull --all-tags parity). Shared by all platforms.
func listRepoTags(ref, username, password string) ([]string, error) {
	return registry.ListRepoTags(context.Background(), ref, username, password)
}

func shortDigest(d string) string {
	if len(d) > 12 {
		return d[:12]
	}
	return d
}

// verifyPulledImage runs cosign verification when requested, removing the
// image on failure so untrusted content is never left behind. Reports
// pull output unless quiet. Returns false on failure (caller exits).
// Shared by the linux and darwin pull paths (windows verifies in-VM).
func verifyPulledImage(ctx context.Context, ref, pulledRef, digest, username, password string, verifyPull bool, verifyKey string, quiet bool) bool {
	if verifyPull {
		if err := signing.VerifyCosignImage(ctx, ref, verifyKey, username, password); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cosign verification failed for %s: %v\n", ref, err)
			image.Remove(ctx, pulledRef) //nolint:errcheck
			return false
		}
		if !quiet {
			fmt.Printf("Verified: %s\n", pulledRef)
		}
	}
	if !quiet {
		fmt.Printf("Pulled: %s@%s\n", pulledRef, shortDigest(digest))
	} else {
		fmt.Println(pulledRef)
	}
	return true
}

// LoginCmd stores registry credentials (docker login parity).
func LoginCmd() *cobra.Command {
	var username, password string
	var passwordStdin bool
	cmd := &cobra.Command{
		Use:   "login [server]",
		Short: "Log in to a registry (stores credentials for pull/push)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			server := "https://index.docker.io/v1/"
			if len(args) == 1 {
				server = args[0]
			}
			if passwordStdin {
				raw, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("login: read password: %w", err)
				}
				password = strings.TrimRight(string(raw), "\r\n")
			}
			if username == "" {
				return fmt.Errorf("login: --username is required")
			}
			if err := registry.Login(server, username, password); err != nil {
				return err
			}
			fmt.Printf("Login Succeeded (%s)\n", registry.RegistryHost(server))
			return nil
		},
	}
	cmd.Flags().StringVarP(&username, "username", "u", "", "Registry username")
	cmd.Flags().StringVarP(&password, "password", "p", "", "Registry password (omit for prompt-free empty)")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "Read password from stdin")
	return cmd
}

// LogoutCmd removes stored registry credentials.
func LogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout [server]",
		Short: "Log out from a registry (removes stored credentials)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			server := "https://index.docker.io/v1/"
			if len(args) == 1 {
				server = args[0]
			}
			if err := registry.Logout(server); err != nil {
				return err
			}
			fmt.Printf("Logged out from %s\n", registry.RegistryHost(server))
			return nil
		},
	}
}

// SearchCmd searches Docker Hub for images.
func SearchCmd() *cobra.Command {
	var limit int
	var filter, format string
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search Docker Hub for images",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := registry.Search(context.Background(), args[0], limit)
			if err != nil {
				return err
			}
			if format != "" {
				for _, r := range results {
					out, err := renderFormat(format, map[string]any{
						"name": r.Name, "description": r.Description,
						"stars": r.Stars, "official": r.Official,
					})
					if err != nil {
						return err
					}
					fmt.Println(out)
				}
				return nil
			}
			fmt.Printf("%-45s %-40s %s\n", "NAME", "DESCRIPTION", "STARS")
			for _, r := range results {
				if filter == "is-official=true" && !r.Official {
					continue
				}
				desc := r.Description
				if len(desc) > 38 {
					desc = desc[:38]
				}
				name := r.Name
				if r.Official {
					name += " [OK]"
				}
				fmt.Printf("%-45s %-40s %d\n", name, desc, r.Stars)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Max number of results (1-100)")
	cmd.Flags().StringVar(&filter, "filter", "", "Filter results (is-official=true)")
	cmd.Flags().StringVar(&format, "format", "", "Format output with a Go template")
	return cmd
}

// ManifestCmd manages OCI manifest lists (docker manifest parity).
func ManifestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "manifest",
		Short: "Manage OCI manifest lists",
	}

	inspect := &cobra.Command{
		Use:   "inspect [image]",
		Short: "Display a remote image manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			username, _ := cmd.Flags().GetString("username")
			password, _ := cmd.Flags().GetString("password")
			info, err := registry.ManifestInspect(cmd.Context(), args[0], username, password)
			if err != nil {
				return err
			}
			out, _ := json.MarshalIndent(info, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}
	inspect.Flags().String("username", "", "Registry username")
	inspect.Flags().String("password", "", "Registry password")

	create := &cobra.Command{
		Use:   "create [list] [image...]",
		Short: "Create a local manifest list",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := registry.ManifestCreate(args[0], args[1:]); err != nil {
				return err
			}
			fmt.Printf("Created manifest list %s\n", args[0])
			return nil
		},
	}

	annotate := &cobra.Command{
		Use:   "annotate [list] [image]",
		Short: "Set platform metadata on a manifest list entry",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			osName, _ := cmd.Flags().GetString("os")
			arch, _ := cmd.Flags().GetString("arch")
			variant, _ := cmd.Flags().GetString("variant")
			if err := registry.ManifestAnnotate(args[0], args[1], osName, arch, variant, nil); err != nil {
				return err
			}
			fmt.Printf("Annotated %s in %s\n", args[1], args[0])
			return nil
		},
	}
	annotate.Flags().String("os", "", "OS (e.g. linux)")
	annotate.Flags().String("arch", "", "Architecture (e.g. arm64)")
	annotate.Flags().String("variant", "", "Variant (e.g. v8)")

	push := &cobra.Command{
		Use:   "push [list]",
		Short: "Push a manifest list to a registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			username, _ := cmd.Flags().GetString("username")
			password, _ := cmd.Flags().GetString("password")
			if err := registry.ManifestPush(cmd.Context(), args[0], username, password); err != nil {
				return err
			}
			fmt.Printf("Pushed manifest list %s\n", args[0])
			return nil
		},
	}
	push.Flags().String("username", "", "Registry username")
	push.Flags().String("password", "", "Registry password")

	rm := &cobra.Command{
		Use:   "rm [list]",
		Short: "Delete a local manifest list",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := registry.ManifestRemove(args[0]); err != nil {
				return err
			}
			fmt.Printf("Removed manifest list %s\n", args[0])
			return nil
		},
	}

	cmd.AddCommand(inspect, create, annotate, push, rm)
	return cmd
}
