//go:build linux

package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/buildx"
	"github.com/thakurprasadrout/thrive/pkg/build"
	"github.com/thakurprasadrout/thrive/pkg/compose"
	"github.com/thakurprasadrout/thrive/pkg/dockerfile"
	"github.com/thakurprasadrout/thrive/pkg/thrivefile"
)

// BuildxCmd provides extended builds (docker buildx parity, native scope).
func BuildxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "buildx",
		Short: "Extended builds (multi-platform records, bake, builders, cache)",
	}

	buildCmd := &cobra.Command{
		Use:   "build [path]",
		Short: "Build from Thrivefile or Dockerfile",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) == 1 {
				path = args[0]
			}
			platform, _ := cmd.Flags().GetString("platform")
			if platform != "" && platform != "linux/amd64" && platform != "linux/arm64" {
				return fmt.Errorf("buildx: cross-arch builds need QEMU (only native linux/amd64, linux/arm64 supported)")
			}
			fileFlag, _ := cmd.Flags().GetString("file")
			tag, _ := cmd.Flags().GetString("tag")
			noCache, _ := cmd.Flags().GetBool("no-cache")

			buildFile, contextDir, isDockerfile, err := resolveBuildFile(path, fileFlag)
			if err != nil {
				return err
			}
			var graph *thrivefile.BuildGraph
			if isDockerfile {
				res, err := dockerfile.ParseFile(buildFile, nil)
				if err != nil {
					return err
				}
				for _, w := range res.Warnings {
					fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
				}
				graph = res.Graph
			} else {
				graph, err = build.ParseThrivefile(buildFile)
				if err != nil {
					return err
				}
			}
			result, err := build.Execute(context.Background(), graph, build.BuildOptions{
				Tag: tag, NoCache: noCache, ContextDir: contextDir,
			})
			if err != nil {
				return err
			}
			fmt.Printf("Build complete: %s (%d steps)\n", result.ImageID, result.Steps)
			return nil
		},
	}
	buildCmd.Flags().String("platform", "", "Target platform (native only)")
	buildCmd.Flags().StringP("file", "f", "", "Build definition file")
	buildCmd.Flags().StringP("tag", "t", "", "Image tag")
	buildCmd.Flags().Bool("no-cache", false, "Do not use the build cache")

	bake := &cobra.Command{
		Use:   "bake [service...]",
		Short: "Build all services from a compose file",
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			cf, err := compose.Load(file)
			if err != nil {
				return err
			}
			return compose.Build(context.Background(), cf, composeDirOf(file), args)
		},
	}
	bake.Flags().StringP("file", "f", "docker-compose.yml", "Compose file path")

	ls := &cobra.Command{
		Use: "ls", Short: "List builders",
		RunE: func(cmd *cobra.Command, args []string) error {
			builders, err := buildx.ListBuilders()
			if err != nil {
				return err
			}
			fmt.Printf("%-20s %-16s %s\n", "NAME", "DRIVER", "CURRENT")
			for _, b := range builders {
				cur := ""
				if b.Current {
					cur = "*"
				}
				fmt.Printf("%-20s %-16s %s\n", b.Name, b.Driver, cur)
			}
			return nil
		},
	}

	create := &cobra.Command{
		Use: "create [name]", Short: "Create a builder", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			driver, _ := cmd.Flags().GetString("driver")
			b, err := buildx.CreateBuilder(args[0], driver, "")
			if err != nil {
				return err
			}
			fmt.Printf("Created builder %s (%s)\n", b.Name, b.Driver)
			return nil
		},
	}
	create.Flags().String("driver", "thrive", "Builder driver")

	rm := &cobra.Command{
		Use: "rm [name]", Short: "Remove a builder", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return buildx.RemoveBuilder(args[0])
		},
	}

	inspectB := &cobra.Command{
		Use: "inspect [name]", Short: "Inspect a builder", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := buildx.InspectBuilder(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Name: %s\nDriver: %s\nCurrent: %v\n", b.Name, b.Driver, b.Current)
			return nil
		},
	}

	use := &cobra.Command{
		Use: "use [name]", Short: "Set the current builder", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return buildx.UseBuilder(args[0])
		},
	}

	du := &cobra.Command{
		Use: "du", Short: "Show build cache usage",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Build cache: %s\n", humanBytes(buildx.CacheUsage("/var/lib/thrive/cache")))
			return nil
		},
	}

	prune := &cobra.Command{
		Use: "prune", Short: "Clear the build cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			reclaimed, err := buildx.PruneCache("/var/lib/thrive/cache")
			if err != nil {
				return err
			}
			fmt.Printf("Reclaimed %s\n", humanBytes(reclaimed))
			return nil
		},
	}

	cmd.AddCommand(buildCmd, bake, ls, create, rm, inspectB, use, du, prune)
	return cmd
}

func composeDirOf(file string) string {
	return dockerfile.ContextDirFor(file)
}
