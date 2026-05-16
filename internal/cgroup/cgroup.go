//go:build linux
// +build linux

package cgroup

import (
	"fmt"
	"os"
	"path/filepath"
)

// Manager handles cgroup v2 resource limits for a single container.
type Manager struct {
	cgroupDir string
}

// New creates a new cgroup manager and initialises the per-container cgroup
// subdirectory under /sys/fs/cgroup/thrive/{containerID}/.
func New(containerID string) (*Manager, error) {
	cgroupDir := filepath.Join("/sys/fs/cgroup/thrive", containerID)
	if err := os.MkdirAll(cgroupDir, 0755); err != nil {
		return nil, fmt.Errorf("cgroup.New: mkdir %s: %w", cgroupDir, err)
	}
	return &Manager{cgroupDir: cgroupDir}, nil
}

// Apply moves pid into this container's cgroup.
func (m *Manager) Apply(pid int) error {
	procsFile := filepath.Join(m.cgroupDir, "cgroup.procs")
	if err := os.WriteFile(procsFile, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		return fmt.Errorf("cgroup.Apply: write pid to %s: %w", procsFile, err)
	}
	return nil
}

// SetMemoryLimit sets the memory hard limit in bytes (cgroup v2 memory.max).
func (m *Manager) SetMemoryLimit(limit int64) error {
	maxFile := filepath.Join(m.cgroupDir, "memory.max")
	if err := os.WriteFile(maxFile, []byte(fmt.Sprintf("%d", limit)), 0644); err != nil {
		return fmt.Errorf("cgroup.SetMemoryLimit: write %s: %w", maxFile, err)
	}
	return nil
}

// SetCPUQuota sets CPU bandwidth as microseconds per 100 ms period
// (cgroup v2 cpu.max format: "<quota> <period>").
func (m *Manager) SetCPUQuota(quota int64) error {
	quotaFile := filepath.Join(m.cgroupDir, "cpu.max")
	if err := os.WriteFile(quotaFile, []byte(fmt.Sprintf("%d 100000", quota)), 0644); err != nil {
		return fmt.Errorf("cgroup.SetCPUQuota: write %s: %w", quotaFile, err)
	}
	return nil
}

// Remove deletes the container's cgroup directory, releasing all limits.
func (m *Manager) Remove() error {
	if err := os.Remove(m.cgroupDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cgroup.Remove: %w", err)
	}
	return nil
}
