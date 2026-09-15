//go:build linux
// +build linux

package checkpoint

import (
	"testing"
)

// TestList_MissingContainer verifies empty results without state.
func TestList_MissingContainer(t *testing.T) {
	checkpointsDirOverride = t.TempDir()
	t.Cleanup(func() { checkpointsDirOverride = "" })

	cps, err := List("no-such-container")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(cps) != 0 {
		t.Errorf("List: got %d, want 0", len(cps))
	}
}

// TestCreate_Validation verifies name validation runs before CRIU use.
func TestCreate_Validation(t *testing.T) {
	checkpointsDirOverride = t.TempDir()
	t.Cleanup(func() { checkpointsDirOverride = "" })

	// Invalid names fail fast regardless of CRIU availability.
	if _, err := Create("ctr", "bad name!"); err == nil {
		t.Error("Create bad name: expected error, got nil")
	}
	// Unknown containers fail (or CRIU missing — either way, an error).
	if _, err := Create("no-such-container", "c1"); err == nil {
		t.Error("Create: expected error, got nil")
	}
}

// TestRemove_Missing verifies removal errors clearly.
func TestRemove_Missing(t *testing.T) {
	checkpointsDirOverride = t.TempDir()
	t.Cleanup(func() { checkpointsDirOverride = "" })

	if err := Remove("ctr", "c1"); err == nil {
		t.Error("Remove: expected error, got nil")
	}
}
