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

// Freeze pauses all processes in the container's cgroup (cgroup v2 freezer).
func (m *Manager) Freeze() error {
	freezeFile := filepath.Join(m.cgroupDir, "cgroup.freeze")
	if err := os.WriteFile(freezeFile, []byte("1"), 0644); err != nil {
		return fmt.Errorf("cgroup.Freeze: write %s: %w", freezeFile, err)
	}
	return nil
}

// Unfreeze resumes all processes in the container's cgroup.
func (m *Manager) Unfreeze() error {
	freezeFile := filepath.Join(m.cgroupDir, "cgroup.freeze")
	if err := os.WriteFile(freezeFile, []byte("0"), 0644); err != nil {
		return fmt.Errorf("cgroup.Unfreeze: write %s: %w", freezeFile, err)
	}
	return nil
}

// SetCPUShares sets CPU weight (cgroup v2 cpu.weight, range 1-10000).
// Docker --cpu-shares (default 1024) maps linearly onto cpu.weight (default 100).
func (m *Manager) SetCPUShares(shares int64) error {
	if shares <= 0 {
		return nil
	}
	weight := shares * 100 / 1024
	if weight < 1 {
		weight = 1
	}
	if weight > 10000 {
		weight = 10000
	}
	weightFile := filepath.Join(m.cgroupDir, "cpu.weight")
	if err := os.WriteFile(weightFile, []byte(fmt.Sprintf("%d", weight)), 0644); err != nil {
		return fmt.Errorf("cgroup.SetCPUShares: write %s: %w", weightFile, err)
	}
	return nil
}

// SetPIDsLimit sets the maximum number of processes (cgroup v2 pids.max).
func (m *Manager) SetPIDsLimit(limit int64) error {
	pidsFile := filepath.Join(m.cgroupDir, "pids.max")
	val := "max"
	if limit > 0 {
		val = fmt.Sprintf("%d", limit)
	}
	if err := os.WriteFile(pidsFile, []byte(val), 0644); err != nil {
		return fmt.Errorf("cgroup.SetPIDsLimit: write %s: %w", pidsFile, err)
	}
	return nil
}
