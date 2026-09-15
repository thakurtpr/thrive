//go:build linux
// +build linux

package cgroup

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Stats holds a point-in-time snapshot of cgroup v2 resource usage.
type Stats struct {
	MemoryCurrent int64
	MemoryMax     int64
	CPUUsageUsec  int64
	PIDsCurrent   int64
	Frozen        bool
}

// ReadStats returns current usage for the given container ID.
// Missing files are treated as zero values so stats work for
// chroot-only or partially-initialised containers.
func ReadStats(containerID string) (Stats, error) {
	dir := filepath.Join("/sys/fs/cgroup/thrive", containerID)
	var s Stats
	s.MemoryCurrent = readIntFile(filepath.Join(dir, "memory.current"))
	s.MemoryMax = readIntFile(filepath.Join(dir, "memory.max"))
	s.PIDsCurrent = readIntFile(filepath.Join(dir, "pids.current"))
	if data, err := os.ReadFile(filepath.Join(dir, "cgroup.freeze")); err == nil {
		s.Frozen = strings.TrimSpace(string(data)) == "1"
	}
	if data, err := os.ReadFile(filepath.Join(dir, "cpu.stat")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			parts := strings.Fields(line)
			if len(parts) == 2 && parts[0] == "usage_usec" {
				if v, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					s.CPUUsageUsec = v
				}
			}
		}
	}
	return s, nil
}

func readIntFile(path string) int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	v, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// ManagerStats returns stats via an existing Manager.
func (m *Manager) Stats() (Stats, error) {
	id := filepath.Base(m.cgroupDir)
	if id == "" || id == "." || id == "/" {
		return Stats{}, fmt.Errorf("cgroup.Stats: invalid cgroup dir %q", m.cgroupDir)
	}
	return ReadStats(id)
}
