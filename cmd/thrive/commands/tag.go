//go:build linux

package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
)

func TagCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tag SOURCE TARGET",
		Short: "Tag a local image with a new name",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]

			parsedSrc, err := name.ParseReference(src)
			if err != nil {
				return fmt.Errorf("tag: invalid source %q: %w", src, err)
			}
			parsedDst, err := name.ParseReference(dst)
			if err != nil {
				return fmt.Errorf("tag: invalid target %q: %w", dst, err)
			}

			imgDir := "/var/lib/thrive/images"
			srcDir := filepath.Join(imgDir, parsedSrc.String())
			dstDir := filepath.Join(imgDir, parsedDst.String())

			data, err := os.ReadFile(filepath.Join(srcDir, "manifest.json"))
			if err != nil {
				return fmt.Errorf("tag: source image %q not found: %w", src, err)
			}
			if err := os.MkdirAll(dstDir, 0755); err != nil {
				return fmt.Errorf("tag: mkdir: %w", err)
			}
			var meta map[string]any
			json.Unmarshal(data, &meta)
			meta["Ref"] = parsedDst.String()
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

			fmt.Printf("Tagged %s as %s\n", parsedSrc.String(), parsedDst.String())
			return nil
		},
	}
}
