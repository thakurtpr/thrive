//go:build !linux

package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func TagCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tag SOURCE TARGET",
		Short: "Tag a local image with a new name",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("tag: home dir: %w", err)
			}
			safeRef := strings.NewReplacer("/", "_", ":", "_", "@", "_").Replace
			imgDir := filepath.Join(home, ".thrive", "images")
			srcDir := filepath.Join(imgDir, safeRef(src))
			dstDir := filepath.Join(imgDir, safeRef(dst))

			data, err := os.ReadFile(filepath.Join(srcDir, "manifest.json"))
			if err != nil {
				return fmt.Errorf("tag: source image %q not found: %w", src, err)
			}
			if err := os.MkdirAll(dstDir, 0755); err != nil {
				return fmt.Errorf("tag: mkdir: %w", err)
			}
			var meta map[string]any
			json.Unmarshal(data, &meta)
			meta["Ref"] = dst
			updated, _ := json.Marshal(meta)
			if err := os.WriteFile(filepath.Join(dstDir, "manifest.json"), updated, 0644); err != nil {
				return fmt.Errorf("tag: write manifest: %w", err)
			}

			srcLayersDir := filepath.Join(srcDir, "layers")
			dstLayersDir := filepath.Join(dstDir, "layers")
			os.Remove(dstLayersDir)
			rel, _ := filepath.Rel(dstDir, srcLayersDir)
			if err := os.Symlink(rel, dstLayersDir); err != nil {
				os.Symlink(srcLayersDir, dstLayersDir)
			}

			fmt.Printf("Tagged %s as %s\n", src, dst)
			return nil
		},
	}
}
