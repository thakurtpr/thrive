//go:build linux

package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/registry"
	"github.com/thakurprasadrout/thrive/pkg/build"
	"github.com/thakurprasadrout/thrive/pkg/dockerfile"
	"github.com/thakurprasadrout/thrive/pkg/thrivefile"
)

var buildTag string

func BuildCmd() *cobra.Command {
	var fileFlag string
	var buildArgs []string
	var noCache bool

	cmd := &cobra.Command{
		Use:   "build -t [tag] [path]",
		Short: "Build from Thrivefile or Dockerfile",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			path := args[0]
			if len(args) > 1 {
				buildTag = args[1]
			}

			buildFile, contextDir, isDockerfile, err := resolveBuildFile(path, fileFlag)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error resolving build file: %v\n", err)
				os.Exit(1)
			}

			argMap, err := parseBuildArgs(buildArgs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing --build-arg: %v\n", err)
				os.Exit(1)
			}

			var graph *thrivefile.BuildGraph
			if isDockerfile {
				res, err := dockerfile.ParseFile(buildFile, argMap)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error parsing Dockerfile: %v\n", err)
					os.Exit(1)
				}
				for _, w := range res.Warnings {
					fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
				}
				graph = res.Graph
				fmt.Printf("Building %s from Dockerfile %s\n", graph.BaseImage, buildFile)
			} else {
				graph, err = build.ParseThrivefile(buildFile)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error parsing Thrivefile: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Building %s from %s\n", graph.BaseImage, buildFile)
			}

			result, err := build.Execute(ctx, graph, build.BuildOptions{
				Tag:        buildTag,
				NoCache:    noCache,
				ContextDir: contextDir,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error building: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Build complete: %s (%d steps)\n", result.ImageID, result.Steps)
		},
	}
	cmd.Flags().StringVarP(&buildTag, "tag", "t", "", "Image tag")
	cmd.Flags().StringVarP(&fileFlag, "file", "f", "", "Build definition file (default: auto-detect Thrivefile/Dockerfile)")
	cmd.Flags().StringArrayVar(&buildArgs, "build-arg", nil, "Build argument (KEY=VALUE)")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "Do not use the build cache")
	return cmd
}

// resolveBuildFile finds the build definition and context dir.
// Returns (buildFile, contextDir, isDockerfile, err).
func resolveBuildFile(path, fileFlag string) (string, string, bool, error) {
	if fileFlag != "" {
		return fileFlag, dockerfile.ContextDirFor(fileFlag), isDockerfilePath(fileFlag), nil
	}
	fi, err := os.Stat(path)
	if err != nil {
		return "", "", false, err
	}
	if !fi.IsDir() {
		return path, dockerfile.ContextDirFor(path), isDockerfileContent(path), nil
	}
	for _, candidate := range []string{"Thrivefile", "thrivefile", "Dockerfile", "dockerfile"} {
		p := filepath.Join(path, candidate)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(path)
			return p, abs, strings.Contains(strings.ToLower(candidate), "dockerfile"), nil
		}
	}
	return "", "", false, fmt.Errorf("no Thrivefile or Dockerfile found in %s (use --file)", path)
}

// isDockerfilePath reports whether a filename looks like a Dockerfile.
func isDockerfilePath(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return strings.Contains(base, "dockerfile")
}

// isDockerfileContent sniffs the first meaningful line for a FROM instruction.
func isDockerfileContent(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		return len(fields) > 0 && strings.ToUpper(fields[0]) == "FROM"
	}
	return false
}

func parseBuildArgs(args []string) (map[string]string, error) {
	out := map[string]string{}
	for _, a := range args {
		k, v, ok := strings.Cut(a, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid --build-arg %q (want KEY=VALUE)", a)
		}
		out[k] = v
	}
	return out, nil
}

func PushCmd() *cobra.Command {
	var username, password string
	var quiet bool

	cmd := &cobra.Command{
		Use:   "push [image]",
		Short: "Push image to registry",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			ref := args[0]
			if !quiet {
				fmt.Printf("Pushing %s ...\n", ref)
			}
			if username == "" {
				if c := registry.StoredAuth(registry.RegistryHost(ref)); c != nil {
					username, password = c.Username, c.Password
				}
			}
			if err := image.Push(ctx, ref, image.PushOptions{
				Username: username,
				Password: password,
			}); err != nil {
				fmt.Fprintf(os.Stderr, "Error pushing image: %v\n", err)
				os.Exit(1)
			}
			if !quiet {
				fmt.Printf("Pushed: %s\n", ref)
			} else {
				fmt.Println(ref)
			}
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "Registry username")
	cmd.Flags().StringVar(&password, "password", "", "Registry password")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Suppress progress output")
	return cmd
}

func PullCmd() *cobra.Command {
	var username, password, platform string
	var quiet, allTags, verifyPull bool
	var verifyKey string

	cmd := &cobra.Command{
		Use:   "pull [image]",
		Short: "Pull image from registry",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			ref := args[0]
			if verifyPull && verifyKey == "" {
				fmt.Fprintf(os.Stderr, "Error: --verify requires --verify-key <cosign.pub>\n")
				os.Exit(1)
			}
			if username == "" {
				if c := registry.StoredAuth(registry.RegistryHost(ref)); c != nil {
					username, password = c.Username, c.Password
				}
			}
			opts := image.PullOptions{Username: username, Password: password, Platform: platform}
			if allTags {
				refs, err := listRepoTags(ref, username, password)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error listing tags for %s: %v\n", ref, err)
					os.Exit(1)
				}
				for _, r := range refs {
					if !quiet {
						fmt.Printf("Pulling %s ...\n", r)
					}
					img, err := image.Pull(ctx, r, opts)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error pulling image: %v\n", err)
						os.Exit(1)
					}
					if !verifyPulledImage(ctx, r, img.Ref, img.Digest, username, password, verifyPull, verifyKey, quiet) {
						os.Exit(1)
					}
				}
				return
			}
			if !quiet {
				fmt.Printf("Pulling %s ...\n", ref)
			}
			img, err := image.Pull(ctx, ref, opts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error pulling image: %v\n", err)
				os.Exit(1)
			}
			if !verifyPulledImage(ctx, ref, img.Ref, img.Digest, username, password, verifyPull, verifyKey, quiet) {
				os.Exit(1)
			}
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
