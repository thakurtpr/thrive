//go:build linux

package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/runtime"
)

func RunCmd() *cobra.Command {
	var detach bool
	var rm bool
	var name string
	var envVars []string
	var secretNames []string

	cmd := &cobra.Command{
		Use:   "run [image] [command...]",
		Short: "Run a container",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			imageRef := args[0]

			fmt.Printf("Pulling image: %s\n", imageRef)
			img, err := image.Pull(ctx, imageRef, image.PullOptions{})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error pulling image: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Pulled: %s\n", img.Ref)

			containerID := name
			if containerID == "" {
				containerID = fmt.Sprintf("thrive-%d", os.Getpid())
			}

			cfg := runtime.ContainerConfig{
				ID:      containerID,
				Image:   img.Ref,
				Command: args[1:],
				Env:     envVars,
				Secrets: secretNames,
			}

			container, err := runtime.Create(ctx, cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating container: %v\n", err)
				os.Exit(1)
			}

			if err := runtime.Start(ctx, container.ID); err != nil {
				fmt.Fprintf(os.Stderr, "Error starting container: %v\n", err)
				os.Exit(1)
			}

			fmt.Println(container.ID)

			if detach {
				// --detach: print ID and return; container runs in background.
				return
			}

			// Foreground: poll until stopped, then optionally remove.
			for {
				state, err := runtime.State(ctx, container.ID)
				if err != nil {
					break
				}
				if state.Status == "stopped" {
					if rm {
						runtime.Delete(ctx, container.ID)
					}
					os.Exit(state.ExitCode)
				}
				time.Sleep(100 * time.Millisecond)
			}
		},
	}

	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "Run container in background")
	cmd.Flags().BoolVar(&rm, "rm", false, "Remove container on exit")
	cmd.Flags().StringVar(&name, "name", "", "Container name")
	cmd.Flags().StringArrayVarP(&envVars, "env", "e", nil, "Set environment variables")
	cmd.Flags().StringArrayVar(&secretNames, "secret", nil, "Secrets to inject")
	return cmd
}
