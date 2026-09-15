//go:build linux

package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/runtime"
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
			buildFirst, _ := cmd.Flags().GetBool("build")
			if buildFirst {
				if err := compose.Build(ctx, cf, filepath.Dir(file), nil); err != nil {
					return err
				}
			}
			scales, err := parseScales(cmd)
			if err != nil {
				return err
			}
			fmt.Printf("Starting services in %s...\n", file)
			return compose.UpScaled(ctx, cf, getProject(), scales)
		},
	}
	up.Flags().StringArray("scale", nil, "Scale a service (service=num)")
	up.Flags().Bool("build", false, "Build services before starting")

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

	build := &cobra.Command{
		Use:   "build [service...]",
		Short: "Build service images",
		RunE: func(cmd *cobra.Command, args []string) error {
			cf, err := compose.Load(file)
			if err != nil {
				return err
			}
			return compose.Build(context.Background(), cf, filepath.Dir(file), args)
		},
	}

	pull := &cobra.Command{
		Use:   "pull [service...]",
		Short: "Pull service images",
		RunE: func(cmd *cobra.Command, args []string) error {
			cf, err := compose.Load(file)
			if err != nil {
				return err
			}
			return compose.Pull(context.Background(), cf, args)
		},
	}

	stop := &cobra.Command{
		Use:   "stop [service...]",
		Short: "Stop service containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cf, err := compose.Load(file)
			if err != nil {
				return err
			}
			timeout, _ := cmd.Flags().GetInt("timeout")
			return compose.Stop(context.Background(), cf, getProject(), args, time.Duration(timeout)*time.Second)
		},
	}
	stop.Flags().IntP("timeout", "t", 10, "Stop timeout in seconds")

	start := &cobra.Command{
		Use:   "start [service...]",
		Short: "Start existing service containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cf, err := compose.Load(file)
			if err != nil {
				return err
			}
			return compose.Start(context.Background(), cf, getProject(), args)
		},
	}

	kill := &cobra.Command{
		Use:   "kill [service...]",
		Short: "Kill service containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cf, err := compose.Load(file)
			if err != nil {
				return err
			}
			sigStr, _ := cmd.Flags().GetString("signal")
			sig := syscall.SIGKILL
			if sigStr != "" {
				if n, err := strconv.Atoi(sigStr); err == nil {
					sig = syscall.Signal(n)
				} else if sigStr == "TERM" {
					sig = syscall.SIGTERM
				}
			}
			return compose.Kill(context.Background(), cf, getProject(), args, sig)
		},
	}
	kill.Flags().StringP("signal", "s", "KILL", "Signal to send (KILL, TERM, or number)")

	restart := &cobra.Command{
		Use:   "restart [service...]",
		Short: "Restart service containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cf, err := compose.Load(file)
			if err != nil {
				return err
			}
			timeout, _ := cmd.Flags().GetInt("timeout")
			return compose.Restart(context.Background(), cf, getProject(), args, time.Duration(timeout)*time.Second)
		},
	}
	restart.Flags().IntP("timeout", "t", 10, "Restart timeout in seconds")

	rm := &cobra.Command{
		Use:   "rm [service...]",
		Short: "Remove stopped service containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cf, err := compose.Load(file)
			if err != nil {
				return err
			}
			force, _ := cmd.Flags().GetBool("force")
			return compose.Rm(context.Background(), cf, getProject(), args, force)
		},
	}
	rm.Flags().BoolP("force", "f", false, "Remove running containers")

	execSvc := &cobra.Command{
		Use:   "exec [service] [command...]",
		Short: "Execute a command in a service container",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			id := getProject() + "-" + args[0] + "-1"
			state, err := runtime.State(ctx, id)
			if err != nil || state.Status != "running" || state.PID == 0 {
				return fmt.Errorf("service %s is not running", args[0])
			}
			nsenterArgs := []string{
				"--target", strconv.Itoa(state.PID),
				"--mount", "--pid", "--ipc", "--uts", "--net",
				"--",
			}
			nsenterArgs = append(nsenterArgs, args[1:]...)
			os.Exit(execInContainerNS(ctx, nsenterArgs))
			return nil
		},
	}

	config := &cobra.Command{
		Use:   "config",
		Short: "Validate and render the compose file",
		RunE: func(cmd *cobra.Command, args []string) error {
			cf, err := compose.Load(file)
			if err != nil {
				return err
			}
			out, err := compose.Config(cf)
			if err != nil {
				return err
			}
			fmt.Print(out)
			return nil
		},
	}

	for _, sub := range []*cobra.Command{build, pull, stop, start, kill, restart, rm, execSvc, config} {
		sub.Flags().StringVarP(&file, "file", "f", "docker-compose.yml", "Compose file path")
		sub.Flags().StringVarP(&project, "project-name", "p", "", "Project name (defaults to directory name)")
		cmd.AddCommand(sub)
	}
	execSvc.Flags().SetInterspersed(false)

	return cmd
}

// parseScales parses --scale service=num flags.
func parseScales(cmd *cobra.Command) (map[string]int, error) {
	raw, _ := cmd.Flags().GetStringArray("scale")
	scales := map[string]int{}
	for _, s := range raw {
		name, num, ok := splitScale(s)
		if !ok {
			return nil, fmt.Errorf("invalid --scale %q (want service=num)", s)
		}
		scales[name] = num
	}
	return scales, nil
}

func splitScale(s string) (string, int, bool) {
	for i, c := range s {
		if c == '=' {
			n, err := strconv.Atoi(s[i+1:])
			if err != nil || n < 0 {
				return "", 0, false
			}
			return s[:i], n, true
		}
	}
	return "", 0, false
}
