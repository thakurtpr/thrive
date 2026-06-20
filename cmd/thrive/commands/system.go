//go:build linux

package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const (
	imagesBaseDir     = "/var/lib/thrive/images"
	containersBaseDir = "/run/thrive/containers"
)

// SystemCmd returns the "system" parent command with info and clean sub-commands.
func SystemCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "System operations",
	}
	cmd.AddCommand(systemInfoCmd(), systemCleanCmd())
	return cmd
}

func systemInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show system info",
		RunE: func(cmd *cobra.Command, args []string) error {
			version := cmd.Root().Version
			if version == "" {
				version = "dev"
			}

			imageCount := countDirs(imagesBaseDir)
			containerCount := countDirs(containersBaseDir)

			fmt.Printf("thrive system information\n")
			fmt.Printf("  Version:       %s\n", version)
			fmt.Printf("  Runtime:       thrive\n")
			fmt.Printf("  OS/Arch:       %s/%s\n", runtime.GOOS, runtime.GOARCH)
			fmt.Printf("  Kernel:        %s\n", readKernel())
			fmt.Printf("  Cgroup driver: %s\n", detectCgroupDriver())
			fmt.Printf("  Storage driver: %s\n", detectStorageDriver())
			fmt.Printf("  Images:        %d\n", imageCount)
			fmt.Printf("  Containers:    %d\n", containerCount)
			return nil
		},
	}
}

func systemCleanCmd() *cobra.Command {
	var allFlag bool
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean up unused resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			removedContainers, err := cleanContainers(allFlag)
			if err != nil {
				return fmt.Errorf("clean containers: %w", err)
			}

			removedImages, err := cleanUnusedImages()
			if err != nil {
				return fmt.Errorf("clean images: %w", err)
			}

			fmt.Printf("Removed %d container(s)\n", removedContainers)
			fmt.Printf("Removed %d image(s)\n", removedImages)
			return nil
		},
	}
	cmd.Flags().BoolVar(&allFlag, "all", false, "Also remove running containers (kills them first)")
	return cmd
}

// countDirs returns the number of subdirectories in dir, or 0 on error.
func countDirs(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() {
			count++
		}
	}
	return count
}

// readKernel reads the kernel version from /proc/version.
func readKernel() string {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return "n/a"
	}
	line := strings.TrimSpace(string(data))
	// /proc/version typically starts with "Linux version 5.x.y ..."
	// Return just the third field (kernel version number) for brevity.
	fields := strings.Fields(line)
	if len(fields) >= 3 {
		return fields[2]
	}
	return line
}

// detectCgroupDriver returns "cgroupv2" if /sys/fs/cgroup/cgroup.controllers
// exists, otherwise "cgroupv1".
func detectCgroupDriver() string {
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err == nil {
		return "cgroupv2"
	}
	return "cgroupv1"
}

// detectStorageDriver checks /proc/mounts for an overlay entry and returns
// "overlay" when found, otherwise "vfs".
func detectStorageDriver() string {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return "n/a"
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		// /proc/mounts format: device mountpoint fstype options dump pass
		if len(fields) >= 3 && fields[2] == "overlay" {
			return "overlay"
		}
	}
	return "vfs"
}

// containerState is a minimal representation of state.json.
type containerState struct {
	Status string `json:"status"`
	PID    int    `json:"pid"`
}

// cleanContainers removes stopped containers. When all is true it also kills
// and removes running containers. Returns the count removed.
func cleanContainers(all bool) (int, error) {
	entries, err := os.ReadDir(containersBaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("readdir %s: %w", containersBaseDir, err)
	}

	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		containerDir := filepath.Join(containersBaseDir, entry.Name())
		statePath := filepath.Join(containerDir, "state.json")

		state, err := readContainerState(statePath)
		if err != nil {
			// No readable state — treat as orphaned and remove.
			if removeErr := os.RemoveAll(containerDir); removeErr == nil {
				removed++
			}
			continue
		}

		if state.Status == "running" {
			if !all {
				continue
			}
			// Kill the process before removing.
			if state.PID > 0 {
				killProcess(state.PID)
			}
		}

		if removeErr := os.RemoveAll(containerDir); removeErr != nil {
			fmt.Fprintf(os.Stderr, "warn: remove %s: %v\n", containerDir, removeErr)
			continue
		}
		removed++
	}
	return removed, nil
}

// cleanUnusedImages removes images that are not referenced by any container.
// Returns the count removed.
func cleanUnusedImages() (int, error) {
	// Collect SafeRef-formatted image names that surviving containers reference.
	used := usedImageRefs()

	entries, err := os.ReadDir(imagesBaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("readdir %s: %w", imagesBaseDir, err)
	}

	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if used[entry.Name()] {
			continue
		}
		imageDir := filepath.Join(imagesBaseDir, entry.Name())
		if removeErr := os.RemoveAll(imageDir); removeErr != nil {
			fmt.Fprintf(os.Stderr, "warn: remove image %s: %v\n", imageDir, removeErr)
			continue
		}
		removed++
	}
	return removed, nil
}

// usedImageRefs returns a set of SafeRef-formatted image names currently
// referenced by containers in containersBaseDir.
func usedImageRefs() map[string]bool {
	used := make(map[string]bool)
	entries, err := os.ReadDir(containersBaseDir)
	if err != nil {
		return used
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		configPath := filepath.Join(containersBaseDir, entry.Name(), "config.json")
		data, err := os.ReadFile(configPath)
		if err != nil {
			continue
		}
		var cfg struct {
			Image string `json:"Image"`
		}
		if err := json.Unmarshal(data, &cfg); err != nil || cfg.Image == "" {
			continue
		}
		// Mirror image.SafeRef transformation (inlined to avoid linux-only import).
		safeRef := strings.NewReplacer("/", "_", ":", "_", "@", "_").Replace(cfg.Image)
		used[safeRef] = true
	}
	return used
}

// readContainerState reads and unmarshals state.json for a container.
func readContainerState(path string) (*containerState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s containerState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// killProcess sends SIGKILL to the given PID, ignoring errors.
func killProcess(pid int) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	proc.Kill() //nolint:errcheck
}
