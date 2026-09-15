//go:build linux
// +build linux

package cgroup

import (
	"os"
	"path/filepath"
	"testing"
)

// testManager returns a Manager rooted at a temp dir (no host cgroup needed).
func testManager(t *testing.T) *Manager {
	t.Helper()
	return &Manager{cgroupDir: t.TempDir()}
}

// TestFreezeUnfreeze_Roundtrip verifies freezer file writes.
func TestFreezeUnfreeze_Roundtrip(t *testing.T) {
	m := testManager(t)
	if err := m.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(m.cgroupDir, "cgroup.freeze"))
	if err != nil || string(data) != "1" {
		t.Errorf("Freeze: file = %q, %v", data, err)
	}
	if err := m.Unfreeze(); err != nil {
		t.Fatalf("Unfreeze: %v", err)
	}
	data, err = os.ReadFile(filepath.Join(m.cgroupDir, "cgroup.freeze"))
	if err != nil || string(data) != "0" {
		t.Errorf("Unfreeze: file = %q, %v", data, err)
	}
}

// TestSetCPUShares verifies weight mapping and file content.
func TestSetCPUShares(t *testing.T) {
	m := testManager(t)
	if err := m.SetCPUShares(0); err != nil {
		t.Fatalf("SetCPUShares(0): %v", err)
	}
	if _, err := os.Stat(filepath.Join(m.cgroupDir, "cpu.weight")); !os.IsNotExist(err) {
		t.Error("SetCPUShares(0): should not write a file")
	}
	if err := m.SetCPUShares(1024); err != nil {
		t.Fatalf("SetCPUShares(1024): %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(m.cgroupDir, "cpu.weight"))
	if string(data) != "100" {
		t.Errorf("SetCPUShares(1024): file = %q, want %q", data, "100")
	}
	if err := m.SetCPUShares(-5); err != nil {
		t.Fatalf("SetCPUShares(-5): %v", err)
	}
}

// TestSetPIDsLimit verifies max/unlimited writes.
func TestSetPIDsLimit(t *testing.T) {
	m := testManager(t)
	if err := m.SetPIDsLimit(128); err != nil {
		t.Fatalf("SetPIDsLimit: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(m.cgroupDir, "pids.max"))
	if string(data) != "128" {
		t.Errorf("pids.max = %q, want %q", data, "128")
	}
	if err := m.SetPIDsLimit(0); err != nil {
		t.Fatalf("SetPIDsLimit(0): %v", err)
	}
	data, _ = os.ReadFile(filepath.Join(m.cgroupDir, "pids.max"))
	if string(data) != "max" {
		t.Errorf("pids.max = %q, want %q", data, "max")
	}
}

// TestManagerStats_EmptyDir verifies zero stats on an empty cgroup dir.
func TestManagerStats_EmptyDir(t *testing.T) {
	m := testManager(t)
	s, err := m.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if s.MemoryCurrent != 0 || s.CPUUsageUsec != 0 || s.Frozen {
		t.Errorf("Stats: got %+v, want zeros", s)
	}
}

// TestManagerStats_ReadsFiles verifies parsing of cgroup files.
func TestManagerStats_ReadsFiles(t *testing.T) {
	m := testManager(t)
	os.WriteFile(filepath.Join(m.cgroupDir, "memory.current"), []byte("1048576\n"), 0644)
	os.WriteFile(filepath.Join(m.cgroupDir, "pids.current"), []byte("3\n"), 0644)
	os.WriteFile(filepath.Join(m.cgroupDir, "cgroup.freeze"), []byte("1\n"), 0644)
	os.WriteFile(filepath.Join(m.cgroupDir, "cpu.stat"), []byte("usage_usec 12345\nuser_usec 100\n"), 0644)
	s, err := m.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if s.MemoryCurrent != 1048576 || s.PIDsCurrent != 3 || s.CPUUsageUsec != 12345 || !s.Frozen {
		t.Errorf("Stats: got %+v", s)
	}
}
