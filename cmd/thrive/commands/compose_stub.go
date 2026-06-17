//go:build !linux

package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func ComposeCmd() *cobra.Command {
	var file string
	var project string

	cmd := &cobra.Command{
		Use:   "compose",
		Short: "Manage multi-container applications (docker-compose compatible)",
	}

	makeSubCmd := func(use, short, bridgeCmd string) *cobra.Command {
		return &cobra.Command{
			Use:   use,
			Short: short,
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := cmd.Context()

				opts := map[string]any{
					"file":    file,
					"project": project,
				}
				if len(args) > 0 {
					opts["services"] = args
				}

				// For "up": embed the compose YAML so the macOS proxy can sync all
				// referenced images to the VM before forwarding to thrived.
				if bridgeCmd == "up" {
					spec, err := os.ReadFile(file)
					if err != nil {
						return fmt.Errorf("compose: cannot read %s: %w", file, err)
					}
					opts["spec"] = string(spec)
				}

				_, err := vm.DialControl(ctx, "compose_"+bridgeCmd, nil, opts)
				return err
			},
		}
	}

	up := makeSubCmd("up", "Create and start all services", "up")
	down := makeSubCmd("down", "Stop and remove all services", "down")
	ps := makeSubCmd("ps", "List service containers", "ps")
	logs := makeSubCmd("logs [service...]", "View output from containers", "logs")

	for _, sub := range []*cobra.Command{up, down, ps, logs} {
		sub.Flags().StringVarP(&file, "file", "f", "docker-compose.yml", "Compose file path")
		sub.Flags().StringVarP(&project, "project-name", "p", "", "Project name")
		cmd.AddCommand(sub)
	}

	return cmd
}
