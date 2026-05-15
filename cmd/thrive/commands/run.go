package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/runtime"
)

func RunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run [image] [command...]",
		Short: "Run a container",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			imageRef := args[0]

			// Pull image
			fmt.Printf("Pulling image: %s\n", imageRef)
			img, err := image.Pull(ctx, imageRef, image.PullOptions{})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error pulling image: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Pulled: %s (%s)\n", img.Ref, img.Digest)

			// Create container
			containerID := fmt.Sprintf("thrive-%d", os.Getpid())
			cfg := runtime.ContainerConfig{
				ID:      containerID,
				Image:   img.Ref,
				Command: args[1:],
			}

			container, err := runtime.Create(ctx, cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating container: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Created container: %s\n", container.ID)

			// Start container
			if err := runtime.Start(ctx, container.ID); err != nil {
				fmt.Fprintf(os.Stderr, "Error starting container: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Container %s started\n", container.ID)
		},
	}
}
