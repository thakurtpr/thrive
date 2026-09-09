//go:build windows

package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/vm"
)

// ImagesCmd and RmiCmd list/remove images stored inside the Thrive VM via
// the control-socket bridge (vm.DialControl) — unlike macOS, Windows VMs
// have normal internet access, so there's no host-side image store to read
// from directly; thrived already implements "images"/"rmi" for the bridge
// (cmd/thrived/exec.go), the same protocol `thrive ps`/`thrive run` use.
func ImagesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "images",
		Short: "List images",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "images", nil, nil)
			if err != nil {
				return err
			}

			var result map[string]any
			json.Unmarshal(data, &result)

			imgs, _ := result["images"].([]any)
			if len(imgs) == 0 {
				fmt.Println("no images — run `thrive pull <image>` first")
				return nil
			}

			fmt.Printf("%-50s %-22s %s\n", "REPOSITORY", "DIGEST", "LAYERS")
			fmt.Println("────────────────────────────────────────────────────────────────────────────────")
			for _, i := range imgs {
				im, ok := i.(map[string]any)
				if !ok {
					continue
				}
				ref, _ := im["ref"].(string)
				digest, _ := im["digest"].(string)
				layers := 0
				if lf, ok := im["layers"].(float64); ok {
					layers = int(lf)
				}
				fmt.Printf("%-50s %-22s %d\n", ref, digest, layers)
			}
			return nil
		},
	}
}

func RmiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rmi [image]",
		Short: "Remove an image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := vm.DialControl(cmd.Context(), "rmi", args, nil)
			if err != nil {
				return err
			}
			fmt.Printf("Image %s removed\n", args[0])
			return nil
		},
	}
}
