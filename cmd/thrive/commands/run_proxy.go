//go:build !linux

package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func RunCmd() *cobra.Command {
	run := &cobra.Command{
		Use:   "run",
		Short: "Run a container",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			opts := containerOpts(cmd, true)

			data, err := vm.DialControl(ctx, "run", args, opts)
			if err != nil {
				return fmt.Errorf("container run failed: %w", err)
			}

			var result map[string]any
			json.Unmarshal(data, &result)

			if id, ok := result["container_id"].(string); ok {
				fmt.Printf("container %s\n", id)
			}

			return nil
		},
	}

	run.Flags().BoolP("detach", "d", false, "Run container in background")
	run.Flags().Bool("rm", false, "Remove container when it exits")
	run.Flags().StringArrayP("env", "e", nil, "Set environment variables")
	run.Flags().StringArray("secret", nil, "Pass secret to container")
	run.Flags().StringArray("config", nil, "Config object mount: name:/container/path")
	run.Flags().String("name", "", "Assign a name to the container")
	run.Flags().StringArrayP("publish", "p", nil, "Publish port(s): host:container[/proto]")
	run.Flags().StringArrayP("volume", "v", nil, "Bind mount: /host:/container")
	run.Flags().String("network", "", "Network mode")
	run.Flags().String("memory", "", "Memory limit (e.g. 512m, 1g)")
	run.Flags().Float64("cpus", 0, "CPU count (e.g. 1.5)")
	run.Flags().Int64("cpu-shares", 0, "CPU shares (relative weight)")
	run.Flags().Int64("pids-limit", 0, "Maximum number of processes")
	run.Flags().String("restart", "", "Restart policy (no, always, on-failure[:max], unless-stopped)")
	run.Flags().BoolP("tty", "t", false, "Allocate a pseudo-TTY")
	run.Flags().BoolP("interactive", "i", false, "Keep stdin open")

	return run
}
