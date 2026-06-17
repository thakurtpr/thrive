//go:build linux

package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func CpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cp [CONTAINER:]SRC [CONTAINER:]DEST",
		Short: "Copy files between a container and the local filesystem",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]

			containerID, srcPath, dstPath, toContainer := parseCpArgsLinux(src, dst)
			if containerID == "" {
				return fmt.Errorf("cp: one argument must be in CONTAINER:PATH form")
			}

			mergedDir := filepath.Join("/run/thrive/containers", containerID, "merged")
			if _, err := os.Stat(mergedDir); os.IsNotExist(err) {
				upperDir := filepath.Join("/run/thrive/containers", containerID, "upper")
				if _, err2 := os.Stat(upperDir); os.IsNotExist(err2) {
					return fmt.Errorf("cp: container %s not found or not started", containerID)
				}
				mergedDir = upperDir
			}

			if toContainer {
				data, err := os.ReadFile(srcPath)
				if err != nil {
					return fmt.Errorf("cp: read %s: %w", srcPath, err)
				}
				target := filepath.Join(mergedDir, dstPath)
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					return fmt.Errorf("cp: mkdir: %w", err)
				}
				if err := os.WriteFile(target, data, 0644); err != nil {
					return fmt.Errorf("cp: write %s: %w", target, err)
				}
				fmt.Printf("Copied %s → %s:%s\n", srcPath, containerID, dstPath)
			} else {
				srcFull := filepath.Join(mergedDir, srcPath)
				data, err := os.ReadFile(srcFull)
				if err != nil {
					return fmt.Errorf("cp: read %s: %w", srcFull, err)
				}
				if err := os.WriteFile(dstPath, data, 0644); err != nil {
					return fmt.Errorf("cp: write %s: %w", dstPath, err)
				}
				fmt.Printf("Copied %s:%s → %s\n", containerID, srcPath, dstPath)
			}
			return nil
		},
	}
}

func parseCpArgsLinux(src, dst string) (containerID, srcPath, dstPath string, toContainer bool) {
	if i := strings.Index(src, ":"); i > 0 {
		return src[:i], src[i+1:], dst, false
	}
	if i := strings.Index(dst, ":"); i > 0 {
		return dst[:i], src, dst[i+1:], true
	}
	return "", src, dst, false
}
