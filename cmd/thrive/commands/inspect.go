//go:build linux

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/inspect"
	"github.com/thakurprasadrout/thrive/internal/runtime"
)

func InspectCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "inspect [container|image]",
		Short: "Display detailed information about a container or image",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			id := args[0]

			info, err := inspectLocal(ctx, id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if format != "" {
				out, err := renderFormat(format, info)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Println(out)
				return
			}
			out, err := json.MarshalIndent(info, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(out))
		},
	}
	cmd.Flags().StringVar(&format, "format", "", "Format output with a Go template")
	return cmd
}

// inspectLocal inspects a container, falling back to image metadata.
func inspectLocal(ctx context.Context, id string) (map[string]any, error) {
	if state, err := runtime.State(ctx, id); err == nil {
		configPath := filepath.Join("/run/thrive/containers", id, "config.json")
		configData, _ := os.ReadFile(configPath)
		var cfg map[string]any
		_ = json.Unmarshal(configData, &cfg)
		return inspect.Container(state.ID, state.Status, state.PID, cfg), nil
	}
	if imgs, err := image.List(ctx); err == nil {
		for _, img := range imgs {
			if img.Ref == id || img.Digest == id {
				layers := make([]inspect.Layer, 0, len(img.Layers))
				for _, l := range img.Layers {
					layers = append(layers, inspect.Layer{Digest: l.Digest, Size: l.Size})
				}
				return inspect.Image(img.Ref, img.Digest, layers), nil
			}
		}
	}
	return nil, fmt.Errorf("no such container or image: %s", id)
}
