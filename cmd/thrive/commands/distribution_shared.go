package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/registry"
)

// LoginCmd stores registry credentials (docker login parity).
func LoginCmd() *cobra.Command {
	var username, password string
	cmd := &cobra.Command{
		Use:   "login [server]",
		Short: "Log in to a registry (stores credentials for pull/push)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			server := "https://index.docker.io/v1/"
			if len(args) == 1 {
				server = args[0]
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
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search Docker Hub for images",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := registry.Search(context.Background(), args[0], limit)
			if err != nil {
				return err
			}
			fmt.Printf("%-45s %-40s %s\n", "NAME", "DESCRIPTION", "STARS")
			for _, r := range results {
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
