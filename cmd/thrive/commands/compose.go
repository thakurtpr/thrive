//go:build linux

package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/pkg/compose"
)

func ComposeCmd() *cobra.Command {
	var file string
	var project string

	cmd := &cobra.Command{
		Use:   "compose",
		Short: "Manage multi-container applications (docker-compose compatible)",
	}

	getProject := func() string {
		if project != "" {
			return project
		}
		dir, _ := filepath.Abs(filepath.Dir(file))
		return filepath.Base(dir)
	}

	up := &cobra.Command{
		Use:   "up",
		Short: "Create and start all services",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cf, err := compose.Load(file)
			if err != nil {
				return err
			}
			fmt.Printf("Starting services in %s...\n", file)
			return compose.Up(ctx, cf, getProject())
		},
	}

	down := &cobra.Command{
		Use:   "down",
		Short: "Stop and remove all services",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cf, err := compose.Load(file)
			if err != nil {
				return err
			}
			fmt.Println("Stopping services...")
			return compose.Down(ctx, cf, getProject())
		},
	}

	ps := &cobra.Command{
		Use:   "ps",
		Short: "List service containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cf, err := compose.Load(file)
			if err != nil {
				return err
			}
			statuses, err := compose.Ps(ctx, cf, getProject())
			if err != nil {
				return err
			}
			fmt.Printf("%-20s %-40s %-12s\n", "NAME", "CONTAINER ID", "STATUS")
			fmt.Printf("%-20s %-40s %-12s\n", "----", "------------", "------")
			for _, s := range statuses {
				fmt.Printf("%-20s %-40s %-12s\n", s.Name, s.ID, s.Status)
			}
			return nil
		},
	}

	logs := &cobra.Command{
		Use:   "logs [service...]",
		Short: "View output from containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cf, err := compose.Load(file)
			if err != nil {
				return err
			}
			proj := getProject()

			targets := args
			if len(targets) == 0 {
				for name := range cf.Services {
					targets = append(targets, name)
				}
			}

			for _, name := range targets {
				id := proj + "-" + name + "-1"
				logPath := "/run/thrive/containers/" + id + "/logs"
				data, readErr := os.ReadFile(logPath)
				if readErr == nil {
					fmt.Printf("==> %s <==\n%s\n", name, string(data))
				}
			}
			return nil
		},
	}

	for _, sub := range []*cobra.Command{up, down, ps, logs} {
		sub.Flags().StringVarP(&file, "file", "f", "docker-compose.yml", "Compose file path")
		sub.Flags().StringVarP(&project, "project-name", "p", "", "Project name (defaults to directory name)")
		cmd.AddCommand(sub)
	}

	return cmd
}
