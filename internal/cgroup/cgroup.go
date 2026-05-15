//go:build linux
// +build linux

package cgroup

import (
	"fmt"
	"os"
	"path/filepath"
)

// Manager handles cgroup v2 resource limits for containers.
type Manager struct {
	path   string
	v2Path string
}

// New creates a new cgroup manager for the given container ID.
func New(containerID string) (*Manager, error) {
	v2Path := "/sys/fs/cgroup"
	if err := os.MkdirAll(v2Path, 0755); err != nil {
		return nil, fmt.Errorf("cgroup.New: mkdir: %w", err)
	}

	return &Manager{
		path:   containerID,
		v2Path: v2Path,
	}, nil
}

// Apply applies the cgroup configuration to a process.
func (m *Manager) Apply(pid int) error {
	procsFile := filepath.Join(m.v2Path, "cgroup.procs")
	if err := os.WriteFile(procsFile, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		return fmt.Errorf("cgroup.Apply: write pid: %w", err)
	}

	return nil
}

// SetMemoryLimit sets the memory limit for the container.
func (m *Manager) SetMemoryLimit(limit int64) error {
	maxFile := filepath.Join(m.v2Path, "memory.max")
	if err := os.WriteFile(maxFile, []byte(fmt.Sprintf("%d", limit)), 0644); err != nil {
		return fmt.Errorf("cgroup.SetMemoryLimit: write: %w", err)
	}
	return nil
}

// SetCPUQuota sets the CPU quota and period.
func (m *Manager) SetCPUQuota(quota int64) error {
	quotaFile := filepath.Join(m.v2Path, "cpu.max")
	quotaStr := fmt.Sprintf("%d 100000", quota)
	if err := os.WriteFile(quotaFile, []byte(quotaStr), 0644); err != nil {
		return fmt.Errorf("cgroup.SetCPUQuota: write: %w", err)
	}
	return nil
}
