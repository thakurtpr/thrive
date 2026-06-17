//go:build linux

package commands

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/runtime"
)

func StopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop [container]",
		Short: "Stop a running container (SIGTERM then SIGKILL)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			id := args[0]

			state, err := runtime.State(ctx, id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: container not found: %v\n", err)
				os.Exit(1)
			}
			if state.Status != "running" {
				fmt.Printf("container %s is not running\n", id)
				return
			}

			runtime.Kill(ctx, id, syscall.SIGTERM)

			deadline := time.Now().Add(10 * time.Second)
			for time.Now().Before(deadline) {
				time.Sleep(200 * time.Millisecond)
				s, err := runtime.State(ctx, id)
				if err != nil || s.Status == "stopped" {
					fmt.Printf("container %s stopped\n", id)
					return
				}
			}

			runtime.Kill(ctx, id, syscall.SIGKILL)
			fmt.Printf("container %s force-stopped\n", id)
		},
	}
}
