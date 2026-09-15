//go:build linux
// +build linux

package cgroup

import (
	"testing"
)

// TestReadStats_MissingContainer verifies ReadStats returns zeros (not error)
// for containers without a cgroup directory.
func TestReadStats_MissingContainer(t *testing.T) {
	s, err := ReadStats("nonexistent-thrive-test-stats")
	if err != nil {
		t.Fatalf("ReadStats: unexpected error: %v", err)
	}
	if s.MemoryCurrent != 0 || s.CPUUsageUsec != 0 || s.PIDsCurrent != 0 {
		t.Errorf("ReadStats: got %+v, want zeros", s)
	}
	if s.Frozen {
		t.Error("ReadStats: Frozen should be false for missing cgroup")
	}
}
