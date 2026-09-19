//go:build !linux

package commands

import (
	encb64 "encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func base64Decode(s string) ([]byte, error) {
	return encb64.StdEncoding.DecodeString(s)
}

func proxySimple(use, short, remote string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := vm.DialControl(cmd.Context(), remote, []string{args[0]}, nil); err != nil {
				return fmt.Errorf("%s failed: %w", remote, err)
			}
			fmt.Printf("container %s: %s ok\n", args[0], remote)
			return nil
		},
	}
}

// CreateCmd proxies container creation to the VM daemon.
func CreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create [image] [command...]",
		Short: "Create a container without starting it",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "create", args, nil)
			if err != nil {
				return fmt.Errorf("create failed: %w", err)
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			if id, ok := result["container_id"].(string); ok {
				fmt.Println(id)
			}
			return nil
		},
	}
}

// PauseCmd proxies pause to the VM daemon.
func PauseCmd() *cobra.Command { return proxySimple("pause [container]", "Pause a container", "pause") }

// UnpauseCmd proxies unpause to the VM daemon.
func UnpauseCmd() *cobra.Command {
	return proxySimple("unpause [container]", "Unpause a container", "unpause")
}

// WaitCmd proxies wait to the VM daemon.
func WaitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "wait [container]",
		Short: "Block until a container stops, then print its exit code",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "wait", []string{args[0]}, nil)
			if err != nil {
				return fmt.Errorf("wait failed: %w", err)
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			fmt.Println(int(result["exit_code"].(float64)))
			return nil
		},
	}
}

// RenameCmd proxies rename to the VM daemon.
func RenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename [old] [new]",
		Short: "Rename a container",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := vm.DialControl(cmd.Context(), "rename", args, nil); err != nil {
				return fmt.Errorf("rename failed: %w", err)
			}
			fmt.Printf("container renamed to %s\n", args[1])
			return nil
		},
	}
}

// StatsCmd proxies stats to the VM daemon. Default is a one-shot snapshot;
// pass --no-stream=false to poll the daemon every 2s until interrupted.
func StatsCmd() *cobra.Command {
	var noStream bool
	cmd := &cobra.Command{
		Use:   "stats [container...]",
		Short: "Display resource usage statistics for containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("stats requires at least one container")
			}
			if noStream {
				return printProxyStats(cmd, args)
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			if err := printProxyStats(cmd, args); err != nil {
				return err
			}
			for {
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
					if err := printProxyStats(cmd, args); err != nil {
						return err
					}
				}
			}
		},
	}
	cmd.Flags().BoolVar(&noStream, "no-stream", true, "Display only the current snapshot (pass --no-stream=false to stream every 2s)")
	return cmd
}

// printProxyStats fetches one stats snapshot per container from the daemon.
func printProxyStats(cmd *cobra.Command, args []string) error {
	for _, id := range args {
		data, err := vm.DialControl(cmd.Context(), "stats", []string{id}, nil)
		if err != nil {
			return fmt.Errorf("stats failed: %w", err)
		}
		var result map[string]any
		json.Unmarshal(data, &result)
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))
	}
	return nil
}

// UpdateCmd proxies update to the VM daemon.
func UpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update [container]",
		Short: "Update resource limits of a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			memory, _ := cmd.Flags().GetString("memory")
			cpuQuota, _ := cmd.Flags().GetInt64("cpu-quota")
			cpuShares, _ := cmd.Flags().GetInt64("cpu-shares")
			pidsLimit, _ := cmd.Flags().GetInt64("pids-limit")
			_, err := vm.DialControl(cmd.Context(), "update", []string{args[0]}, map[string]any{
				"memory": memory, "cpu_quota": cpuQuota, "cpu_shares": cpuShares, "pids_limit": pidsLimit,
			})
			if err != nil {
				return fmt.Errorf("update failed: %w", err)
			}
			fmt.Printf("container %s updated\n", args[0])
			return nil
		},
	}
	cmd.Flags().String("memory", "", "Memory limit (e.g. 512m, 1g)")
	cmd.Flags().Int64("cpu-quota", 0, "CPU quota in microseconds per 100ms period")
	cmd.Flags().Int64("cpu-shares", 0, "CPU shares (relative weight)")
	cmd.Flags().Int64("pids-limit", 0, "Maximum number of processes")
	return cmd
}

// TopCmd proxies top to the VM daemon.
func TopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "top [container]",
		Short: "Display running processes in a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "top", []string{args[0]}, nil)
			if err != nil {
				return fmt.Errorf("top failed: %w", err)
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			if procs, ok := result["processes"].([]any); ok {
				fmt.Printf("%-8s %-8s %s\n", "PID", "PPID", "CMD")
				for _, p := range procs {
					pm, _ := p.(map[string]any)
					fmt.Printf("%-8v %-8v %v\n", pm["pid"], pm["ppid"], pm["cmd"])
				}
			}
			return nil
		},
	}
}

// PortCmd proxies port to the VM daemon.
func PortCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "port [container] [private-port]",
		Short: "List port mappings for a container",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "port", args, nil)
			if err != nil {
				return fmt.Errorf("port failed: %w", err)
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			if ports, ok := result["ports"].([]any); ok {
				for _, p := range ports {
					pm, _ := p.(map[string]any)
					fmt.Printf("%v/%v -> 0.0.0.0:%v\n", pm["container_port"], pm["protocol"], pm["host_port"])
				}
			}
			return nil
		},
	}
}

// DiffCmd proxies diff to the VM daemon.
func DiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff [container]",
		Short: "Inspect changes to files in a container's filesystem",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "diff", []string{args[0]}, nil)
			if err != nil {
				return fmt.Errorf("diff failed: %w", err)
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			if changes, ok := result["changes"].([]any); ok {
				for _, c := range changes {
					cm, _ := c.(map[string]any)
					fmt.Printf("%v %v\n", cm["kind"], cm["path"])
				}
			}
			return nil
		},
	}
}

// ExportCmd proxies export to the VM daemon.
func ExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export [container]",
		Short: "Export a container's filesystem as a tar archive",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			output, _ := cmd.Flags().GetString("output")
			data, err := vm.DialControl(cmd.Context(), "export", []string{args[0]}, nil)
			if err != nil {
				return fmt.Errorf("export failed: %w", err)
			}
			var result map[string]any
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("export: parse response: %w", err)
			}
			encoded, _ := result["data"].(string)
			raw, err := base64Decode(encoded)
			if err != nil {
				return fmt.Errorf("export: decode: %w", err)
			}
			if output == "" || output == "-" {
				_, err = os.Stdout.Write(raw)
				return err
			}
			return os.WriteFile(output, raw, 0644)
		},
	}
	cmd.Flags().StringP("output", "o", "", "Write to file instead of stdout")
	return cmd
}

// CommitCmd proxies commit to the VM daemon.
func CommitCmd() *cobra.Command {
	var message, author string
	var pause bool
	cmd := &cobra.Command{
		Use:   "commit [container] [new-image]",
		Short: "Create a new image from a container's changes",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := map[string]any{"message": message, "author": author, "pause": pause}
			if _, err := vm.DialControl(cmd.Context(), "commit", args, opts); err != nil {
				return fmt.Errorf("commit failed: %w", err)
			}
			fmt.Printf("committed %s as %s\n", args[0], args[1])
			return nil
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "", "Commit message (recorded in image manifest)")
	cmd.Flags().StringVarP(&author, "author", "a", "", "Author (recorded in image manifest)")
	cmd.Flags().BoolVarP(&pause, "pause", "p", true, "Pause the container during commit")
	return cmd
}
