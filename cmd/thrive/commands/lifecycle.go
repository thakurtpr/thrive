//go:build linux

package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/runtime"
)

// CreateCmd creates a container without starting it (docker create parity).
func CreateCmd() *cobra.Command {
	var name string
	var envVars []string
	var secretNames []string
	var configSpecs []string
	var portSpecs []string
	var volumeSpecs []string
	var netMode string

	cmd := &cobra.Command{
		Use:   "create [image] [command...]",
		Short: "Create a container without starting it",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			imageRef := args[0]
			containerArgs := args[1:]
			if len(containerArgs) > 0 && containerArgs[0] == "--" {
				containerArgs = containerArgs[1:]
			}
			img, err := image.Pull(ctx, imageRef, image.PullOptions{})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error pulling image: %v\n", err)
				os.Exit(1)
			}
			containerID := name
			if containerID == "" {
				containerID = fmt.Sprintf("thrive-%d", os.Getpid())
			}
			ports, err := parsePortSpecs(portSpecs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing ports: %v\n", err)
				os.Exit(1)
			}
			mounts, err := parseVolumeSpecs(volumeSpecs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing volumes: %v\n", err)
				os.Exit(1)
			}
			cfg := runtime.ContainerConfig{
				ID:          containerID,
				Image:       img.Ref,
				Command:     containerArgs,
				Env:         envVars,
				Secrets:     secretNames,
				Configs:     parseConfigSpecs(configSpecs),
				Ports:       ports,
				Mounts:      mounts,
				NetworkMode: netMode,
			}
			applyResourceFlags(cmd, &cfg)
			container, err := runtime.Create(ctx, cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating container: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(container.ID)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Container name")
	cmd.Flags().StringArrayVarP(&envVars, "env", "e", nil, "Set environment variables")
	cmd.Flags().StringArrayVar(&secretNames, "secret", nil, "Secrets to inject")
	cmd.Flags().StringArrayVar(&configSpecs, "config", nil, "Config object mount: name:/container/path")
	cmd.Flags().StringArrayVarP(&portSpecs, "publish", "p", nil, "Publish port(s): host:container[/proto]")
	cmd.Flags().StringArrayVarP(&volumeSpecs, "volume", "v", nil, "Bind mount: /host:/container")
	cmd.Flags().StringVar(&netMode, "network", "", "Network mode (host, none, or default bridge)")
	resourceFlags(cmd, new(string), new(string), new(float64), new(int64), new(int64))
	cmd.Flags().SetInterspersed(false)
	return cmd
}

// PauseCmd freezes a running container (docker pause parity).
func PauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pause [container]",
		Short: "Pause all processes in a container (cgroup freezer)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := runtime.Pause(context.Background(), args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("container %s paused\n", args[0])
		},
	}
}

// UnpauseCmd resumes a paused container.
func UnpauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unpause [container]",
		Short: "Unpause a paused container",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := runtime.Unpause(context.Background(), args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("container %s unpaused\n", args[0])
		},
	}
}

// WaitCmd blocks until the container stops and prints its exit code.
func WaitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "wait [container]",
		Short: "Block until a container stops, then print its exit code",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			code, err := runtime.Wait(context.Background(), args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(code)
		},
	}
}

// RenameCmd renames a container.
func RenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename [old] [new]",
		Short: "Rename a container",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			if err := runtime.Rename(context.Background(), args[0], args[1]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("container renamed to %s\n", args[1])
		},
	}
}

// StatsCmd shows live resource usage (docker stats parity).
// Default is a one-shot snapshot (--no-stream=true, thrive's historical
// behavior). Pass --no-stream=false to poll every 2s until interrupted.
func StatsCmd() *cobra.Command {
	var noStream bool
	cmd := &cobra.Command{
		Use:   "stats [container...]",
		Short: "Display resource usage statistics for containers",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			if len(args) == 0 {
				fmt.Fprintf(os.Stderr, "Error: stats requires at least one container\n")
				os.Exit(1)
			}
			if noStream {
				printStatsSnapshot(ctx, args)
				return
			}
			streamCtx, stop := signal.NotifyContext(ctx, os.Interrupt)
			defer stop()
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			printStatsSnapshot(ctx, args)
			for {
				select {
				case <-streamCtx.Done():
					return
				case <-ticker.C:
					printStatsSnapshot(ctx, args)
				}
			}
		},
	}
	cmd.Flags().BoolVar(&noStream, "no-stream", true, "Display only the current snapshot (pass --no-stream=false to stream every 2s)")
	return cmd
}

// printStatsSnapshot prints one stats table for ids; errors go to stderr
// per-container so one missing container doesn't hide the rest. Rendering
// delegates to the shared helper so Linux and the VM-daemon proxy print
// identical tables.
func printStatsSnapshot(ctx context.Context, ids []string) {
	var rows []statsRow
	for _, id := range ids {
		s, err := runtime.Stats(ctx, id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}
		rows = append(rows, statsRow{
			ID: s.ID, Status: s.Status,
			MemCurrent: s.MemoryCurrent, MemLimit: s.MemoryLimit,
			PIDs: s.PIDsCurrent, CPUUsec: s.CPUUsageUsec,
		})
	}
	formatStatsTable(os.Stdout, rows)
}

// UpdateCmd updates resource limits of a container live.
func UpdateCmd() *cobra.Command {
	var memStr string
	var cpuQuota int64
	var cpuShares int64
	var pidsLimit int64
	cmd := &cobra.Command{
		Use:   "update [container]",
		Short: "Update resource limits of a container",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var memBytes int64
			if memStr != "" {
				v, err := parseMemory(memStr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: invalid --memory %q: %v\n", memStr, err)
					os.Exit(1)
				}
				memBytes = v
			}
			err := runtime.Update(context.Background(), args[0], runtime.UpdateOptions{
				MemoryLimit: memBytes,
				CPUQuota:    cpuQuota,
				CPUShares:   cpuShares,
				PIDsLimit:   pidsLimit,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("container %s updated\n", args[0])
		},
	}
	cmd.Flags().StringVar(&memStr, "memory", "", "Memory limit (e.g. 512m, 1g)")
	cmd.Flags().Int64Var(&cpuQuota, "cpu-quota", 0, "CPU quota in microseconds per 100ms period")
	cmd.Flags().Int64Var(&cpuShares, "cpu-shares", 0, "CPU shares (relative weight)")
	cmd.Flags().Int64Var(&pidsLimit, "pids-limit", 0, "Maximum number of processes")
	return cmd
}

// TopCmd lists processes running in a container.
func TopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "top [container]",
		Short: "Display running processes in a container",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			procs, err := runtime.Top(context.Background(), args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("%-8s %-8s %s\n", "PID", "PPID", "CMD")
			for _, p := range procs {
				fmt.Printf("%-8d %-8d %s\n", p.PID, p.PPID, p.Cmd)
			}
		},
	}
}

// PortCmd lists port mappings for a container.
func PortCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "port [container] [private-port]",
		Short: "List port mappings for a container",
		Args:  cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			ports, err := runtime.ContainerPorts(context.Background(), args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			filter := ""
			if len(args) == 2 {
				filter = args[1]
			}
			for _, pm := range ports {
				if filter != "" && strconv.Itoa(pm.ContainerPort) != filter {
					continue
				}
				fmt.Printf("%d/%s -> 0.0.0.0:%d\n", pm.ContainerPort, pm.Protocol, pm.HostPort)
			}
		},
	}
}

// DiffCmd shows filesystem changes in the container's writable layer.
func DiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff [container]",
		Short: "Inspect changes to files in a container's filesystem",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			changes, err := runtime.Diff(context.Background(), args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			for _, c := range changes {
				fmt.Printf("%s %s\n", c.Kind, c.Path)
			}
		},
	}
}

// ExportCmd exports a container's filesystem as a tar archive.
func ExportCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "export [container]",
		Short: "Export a container's filesystem as a tar archive",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var w *os.File
			if output == "" || output == "-" {
				w = os.Stdout
			} else {
				f, err := os.Create(output)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				defer f.Close()
				w = f
			}
			if err := runtime.Export(context.Background(), args[0], w); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write to file instead of stdout")
	return cmd
}

// CommitCmd snapshots a container as a new image.
func CommitCmd() *cobra.Command {
	var message, author string
	var pause bool
	cmd := &cobra.Command{
		Use:   "commit [container] [new-image]",
		Short: "Create a new image from a container's changes",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			if err := runtime.CommitWithOptions(context.Background(), args[0], args[1], runtime.CommitOptions{
				Author: author, Message: message, Pause: pause,
			}); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("committed %s as %s\n", args[0], args[1])
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "", "Commit message (recorded in image manifest)")
	cmd.Flags().StringVarP(&author, "author", "a", "", "Author (recorded in image manifest)")
	cmd.Flags().BoolVarP(&pause, "pause", "p", true, "Pause the container during commit")
	return cmd
}

func parseMemory(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	mult := int64(1)
	for _, suffix := range []struct {
		suf string
		m   int64
	}{
		{"g", 1 << 30}, {"gb", 1 << 30},
		{"m", 1 << 20}, {"mb", 1 << 20},
		{"k", 1 << 10}, {"kb", 1 << 10},
		{"b", 1},
	} {
		if strings.HasSuffix(s, suffix.suf) {
			mult = suffix.m
			s = strings.TrimSuffix(s, suffix.suf)
			break
		}
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, err
	}
	return n * mult, nil
}
