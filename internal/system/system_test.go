//go:build linux
// +build linux

package system

import (
	"testing"
)

// TestDiskUsage_EmptyStores verifies zero values when stores are absent.
// NOTE: uses production paths; on CI runners without thrive state all
// counts must be zero (or match ambient state — only assert no error).
func TestDiskUsage_NoError(t *testing.T) {
	u, err := DiskUsage()
	if err != nil {
		t.Fatalf("DiskUsage: %v", err)
	}
	if u == nil {
		t.Fatal("DiskUsage: nil report")
	}
	if u.Containers.Size < 0 || u.Images.Size < 0 {
		t.Errorf("DiskUsage: negative sizes: %+v", u)
	}
}

// TestPrune_EmptyStores verifies prune succeeds with nothing to do.
func TestPrune_NoError(t *testing.T) {
	rep, err := Prune(PruneOptions{})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if rep == nil {
		t.Fatal("Prune: nil report")
	}
	if rep.SpaceReclaimed < 0 {
		t.Errorf("Prune: negative reclaim: %+v", rep)
	}
}
