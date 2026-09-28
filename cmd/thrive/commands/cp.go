//go:build linux

package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func CpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cp [CONTAINER:]SRC [CONTAINER:]DEST",
		Short: "Copy files or directories between a container and the local filesystem",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, dst := args[0], args[1]

			containerID, srcPath, dstPath, toContainer := parseCpArgs(src, dst)
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
				return cpToContainer(srcPath, filepath.Join(mergedDir, dstPath), containerID, dstPath)
			}
			return cpFromContainer(filepath.Join(mergedDir, srcPath), dstPath, containerID, srcPath)
		},
	}
}

// cpToContainer copies a host file or directory tree into the container.
func cpToContainer(srcPath, target, containerID, dstPath string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("cp: read %s: %w", srcPath, err)
	}
	if !info.IsDir() {
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("cp: read %s: %w", srcPath, err)
		}
		if strings.HasSuffix(dstPath, "/") {
			target = filepath.Join(target, filepath.Base(srcPath))
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fmt.Errorf("cp: mkdir: %w", err)
		}
		if err := os.WriteFile(target, data, 0644); err != nil {
			return fmt.Errorf("cp: write %s: %w", target, err)
		}
		fmt.Printf("Copied %s → %s:%s\n", srcPath, containerID, dstPath)
		return nil
	}
	if err := copyTree(srcPath, target); err != nil {
		return fmt.Errorf("cp: copy dir: %w", err)
	}
	fmt.Printf("Copied %s → %s:%s\n", srcPath, containerID, dstPath)
	return nil
}

// cpFromContainer copies a container file or directory tree to the host.
func cpFromContainer(srcFull, dstPath, containerID, srcPath string) error {
	info, err := os.Stat(srcFull)
	if err != nil {
		return fmt.Errorf("cp: read %s: %w", srcFull, err)
	}
	if !info.IsDir() {
		data, err := os.ReadFile(srcFull)
		if err != nil {
			return fmt.Errorf("cp: read %s: %w", srcFull, err)
		}
		target := dstPath
		if st, err := os.Stat(dstPath); err == nil && st.IsDir() {
			target = filepath.Join(dstPath, filepath.Base(srcFull))
		}
		if err := os.WriteFile(target, data, 0644); err != nil {
			return fmt.Errorf("cp: write %s: %w", target, err)
		}
		fmt.Printf("Copied %s:%s → %s\n", containerID, srcPath, target)
		return nil
	}
	target := dstPath
	if st, err := os.Stat(dstPath); err == nil && st.IsDir() {
		target = filepath.Join(dstPath, filepath.Base(srcFull))
	}
	if err := copyTree(srcFull, target); err != nil {
		return fmt.Errorf("cp: copy dir: %w", err)
	}
	fmt.Printf("Copied %s:%s → %s\n", containerID, srcPath, target)
	return nil
}

// copyTree recursively copies src dir into dst (created if missing).
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return os.MkdirAll(dst, 0755)
		}
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, fi.Mode())
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			os.Remove(target) //nolint:errcheck
			return os.Symlink(link, target)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fi.Mode())
		if err != nil {
			_ = in.Close()
			return err
		}
		_, err = io.Copy(out, in)
		cerr := out.Close()
		_ = in.Close()
		if err != nil {
			return err
		}
		return cerr
	})
}
