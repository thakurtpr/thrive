//go:build !linux

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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

	getProject := func() string {
		if project != "" {
			return project
		}
		dir, _ := filepath.Abs(filepath.Dir(file))
		return filepath.Base(dir)
	}

	// specOpts embeds the compose file so the daemon can operate without
	// host filesystem access.
	specOpts := func(extra map[string]any) (map[string]any, error) {
		spec, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("compose: cannot read %s: %w", file, err)
		}
		opts := map[string]any{
			"file":    file,
			"project": getProject(),
			"spec":    string(spec),
		}
		for k, v := range extra {
			opts[k] = v
		}
		return opts, nil
	}

	daemonSub := func(use, short, bridgeCmd string) *cobra.Command {
		return &cobra.Command{
			Use:   use,
			Short: short,
			RunE: func(cmd *cobra.Command, args []string) error {
				extra := map[string]any{}
				if len(args) > 0 {
					extra["services"] = args
				}
				opts, err := specOpts(extra)
				if err != nil {
					return err
				}
				_, err = vm.DialControl(cmd.Context(), "compose_"+bridgeCmd, nil, opts)
				return err
			},
		}
	}

	up := &cobra.Command{
		Use:   "up",
		Short: "Create and start all services",
		RunE: func(cmd *cobra.Command, args []string) error {
			if build, _ := cmd.Flags().GetBool("build"); build {
				return fmt.Errorf("compose up --build: requires Linux runtime — build contexts are not synced to the VM")
			}
			scales, _ := cmd.Flags().GetStringArray("scale")
			extra := map[string]any{}
			if len(args) > 0 {
				extra["services"] = args
			}
			if len(scales) > 0 {
				extra["scale"] = scales
			}
			opts, err := specOpts(extra)
			if err != nil {
				return err
			}
			_, err = vm.DialControl(cmd.Context(), "compose_up", nil, opts)
			return err
		},
	}
	up.Flags().StringArray("scale", nil, "Scale a service (service=num)")
	up.Flags().Bool("build", false, "Build services before starting (requires Linux runtime)")
	down := daemonSub("down", "Stop and remove all services", "down")
	ps := daemonSub("ps", "List service containers", "ps")
	logs := daemonSub("logs [service...]", "View output from containers", "logs")
	pull := daemonSub("pull [service...]", "Pull service images", "pull")
	config := &cobra.Command{
		Use:   "config",
		Short: "Validate and render the compose file",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := specOpts(nil)
			if err != nil {
				return err
			}
			data, err := vm.DialControl(cmd.Context(), "compose_config", nil, opts)
			if err != nil {
				return err
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			if cfg, ok := result["config"].(string); ok {
				fmt.Print(cfg)
			}
			return nil
		},
	}
	build := &cobra.Command{
		Use:   "build [service...]",
		Short: "Build service images",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("compose build: requires Linux runtime — build contexts are not synced to the VM")
		},
	}

	// serviceContainerIDs resolves service replicas via the daemon ps.
	serviceContainerIDs := func(ctx context.Context, service string) ([]string, error) {
		opts, err := specOpts(nil)
		if err != nil {
			return nil, err
		}
		data, err := vm.DialControl(ctx, "compose_ps", nil, opts)
		if err != nil {
			return nil, err
		}
		var result map[string]any
		json.Unmarshal(data, &result)
		var ids []string
		if containers, ok := result["containers"].([]any); ok {
			for _, c := range containers {
				cm, _ := c.(map[string]any)
				if cm["service"] == service {
					if id, ok := cm["id"].(string); ok {
						ids = append(ids, id)
					}
				}
			}
		}
		if len(ids) == 0 {
			ids = []string{getProject() + "-" + service + "-1"}
		}
		return ids, nil
	}

	forEachContainerWithOpts := func(ctx context.Context, bridgeCmd string, args []string, opts map[string]any) error {
		services := args
		if len(services) == 0 {
			// No targets: operate on all services like Linux
			// (filterServices falls back to all on empty).
			baseOpts, err := specOpts(nil)
			if err != nil {
				return err
			}
			spec, _ := baseOpts["spec"].(string)
			services, err = parseComposeServiceNames(spec)
			if err != nil {
				return err
			}
		}
		for _, svc := range services {
			ids, err := serviceContainerIDs(ctx, svc)
			if err != nil {
				return err
			}
			for _, id := range ids {
				if _, err := vm.DialControl(ctx, bridgeCmd, []string{id}, opts); err != nil {
					return fmt.Errorf("%s %s: %w", bridgeCmd, svc, err)
				}
			}
		}
		return nil
	}

	forEachContainer := func(bridgeCmd string) func(cmd *cobra.Command, args []string) error {
		return func(cmd *cobra.Command, args []string) error {
			return forEachContainerWithOpts(cmd.Context(), bridgeCmd, args, nil)
		}
	}

	stop := &cobra.Command{Use: "stop [service...]", Short: "Stop service containers", RunE: func(cmd *cobra.Command, args []string) error {
		timeout, _ := cmd.Flags().GetInt("timeout")
		return forEachContainerWithOpts(cmd.Context(), "stop", args, map[string]any{"timeout": timeout})
	}}
	stop.Flags().IntP("timeout", "t", 10, "Stop timeout in seconds")
	start := &cobra.Command{Use: "start [service...]", Short: "Start service containers", RunE: forEachContainer("start")}
	kill := &cobra.Command{Use: "kill [service...]", Short: "Kill service containers", RunE: func(cmd *cobra.Command, args []string) error {
		signal, _ := cmd.Flags().GetString("signal")
		return forEachContainerWithOpts(cmd.Context(), "kill", args, map[string]any{"signal": signal})
	}}
	kill.Flags().StringP("signal", "s", "KILL", "Signal to send (KILL, TERM, or number)")
	rm := &cobra.Command{Use: "rm [service...]", Short: "Remove service containers", RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		return forEachContainerWithOpts(cmd.Context(), "rm", args, map[string]any{"force": force})
	}}
	rm.Flags().Bool("force", false, "Remove running containers (no -f shorthand: -f is --file on compose subs)")
	restart := &cobra.Command{Use: "restart [service...]", Short: "Restart service containers", RunE: func(cmd *cobra.Command, args []string) error {
		timeout, _ := cmd.Flags().GetInt("timeout")
		return forEachContainerWithOpts(cmd.Context(), "restart", args, map[string]any{"timeout": timeout})
	}}
	restart.Flags().IntP("timeout", "t", 10, "Restart timeout in seconds")
	execSvc := &cobra.Command{
		Use:   "exec [service] [command...]",
		Short: "Execute a command in a service container",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids, err := serviceContainerIDs(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			_, err = vm.DialControl(cmd.Context(), "exec", append([]string{ids[0]}, args[1:]...), nil)
			return err
		},
	}

	subs := []*cobra.Command{up, down, ps, logs, pull, config, build, stop, start, kill, rm, restart, execSvc}
	for _, sub := range subs {
		sub.Flags().StringVarP(&file, "file", "f", "docker-compose.yml", "Compose file path")
		sub.Flags().StringVarP(&project, "project-name", "p", "", "Project name")
		cmd.AddCommand(sub)
	}

	return cmd
}
