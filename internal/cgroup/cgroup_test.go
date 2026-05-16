//go:build linux
// +build linux

package cgroup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNew_SkipIfNoPermission attempts to create a real cgroup under
// /sys/fs/cgroup/thrive and skips if the process lacks the necessary
// privilege, which is expected in most CI environments.
func TestNew_SkipIfNoPermission(t *testing.T) {
	// Act
	mgr, err := New("thrive-test-container")

	// Assert
	if err != nil {
		if os.IsPermission(err) || strings.Contains(err.Error(), "permission denied") ||
			strings.Contains(err.Error(), "read-only") {
			t.Skipf("skipping: insufficient permission to create cgroup: %v", err)
		}
		t.Fatalf("New: unexpected error: %v", err)
	}

	if mgr == nil {
		t.Fatal("New: returned nil manager without error")
	}

	// Cleanup best-effort.
	_ = mgr.Remove()
}

// TestSetMemoryLimit_WritesFile creates a Manager pointing at a temp directory
// instead of /sys/fs/cgroup, then calls SetMemoryLimit and asserts the file
// content matches the expected byte representation.
func TestSetMemoryLimit_WritesFile(t *testing.T) {
	// Arrange — bypass New() to avoid needing /sys/fs/cgroup access.
	dir := t.TempDir()
	mgr := &Manager{cgroupDir: dir}

	const limit int64 = 134217728 // 128 MiB

	// Act
	if err := mgr.SetMemoryLimit(limit); err != nil {
		t.Fatalf("SetMemoryLimit: unexpected error: %v", err)
	}

	// Assert
	data, err := os.ReadFile(filepath.Join(dir, "memory.max"))
	if err != nil {
		t.Fatalf("ReadFile memory.max: %v", err)
	}

	want := fmt.Sprintf("%d", limit)
	if got := strings.TrimSpace(string(data)); got != want {
		t.Errorf("memory.max content: got %q, want %q", got, want)
	}
}

// TestSetCPUQuota_WritesFile verifies SetCPUQuota writes the correct
// "quota period" pair to cpu.max in the cgroup directory.
func TestSetCPUQuota_WritesFile(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	mgr := &Manager{cgroupDir: dir}

	const quota int64 = 50000 // 50 ms per 100 ms period → 50% CPU

	// Act
	if err := mgr.SetCPUQuota(quota); err != nil {
		t.Fatalf("SetCPUQuota: unexpected error: %v", err)
	}

	// Assert
	data, err := os.ReadFile(filepath.Join(dir, "cpu.max"))
	if err != nil {
		t.Fatalf("ReadFile cpu.max: %v", err)
	}

	want := fmt.Sprintf("%d 100000", quota)
	if got := strings.TrimSpace(string(data)); got != want {
		t.Errorf("cpu.max content: got %q, want %q", got, want)
	}
}
