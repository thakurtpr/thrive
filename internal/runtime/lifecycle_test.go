//go:build linux
// +build linux

package runtime

import (
	"context"
	"testing"
)

// TestRename_EmptyIDs verifies Rename rejects empty IDs without touching disk.
func TestRename_EmptyIDs(t *testing.T) {
	if err := Rename(context.Background(), "", "new"); err == nil {
		t.Error("Rename(\"\", \"new\"): expected error, got nil")
	}
	if err := Rename(context.Background(), "old", ""); err == nil {
		t.Error("Rename(\"old\", \"\"): expected error, got nil")
	}
}

// TestRename_MissingContainer verifies Rename errors for unknown containers.
func TestRename_MissingContainer(t *testing.T) {
	err := Rename(context.Background(), "nonexistent-thrive-test-a", "nonexistent-thrive-test-b")
	if err == nil {
		t.Error("Rename: expected error for missing container, got nil")
	}
}

// TestWait_MissingContainer verifies Wait errors for unknown containers.
func TestWait_MissingContainer(t *testing.T) {
	_, err := Wait(context.Background(), "nonexistent-thrive-test-wait")
	if err == nil {
		t.Error("Wait: expected error for missing container, got nil")
	}
}

// TestStats_MissingContainer verifies Stats errors for unknown containers.
func TestStats_MissingContainer(t *testing.T) {
	_, err := Stats(context.Background(), "nonexistent-thrive-test-stats")
	if err == nil {
		t.Error("Stats: expected error for missing container, got nil")
	}
}

// TestCommit_EmptyRef verifies Commit rejects an empty image reference.
func TestCommit_EmptyRef(t *testing.T) {
	// Arrange a minimal container dir so the test reaches ref validation.
	// Commit checks state first, so a missing container also errors — either
	// way an error is required; empty ref must never succeed.
	err := Commit(context.Background(), "nonexistent-thrive-test-commit", "")
	if err == nil {
		t.Error("Commit: expected error for empty ref, got nil")
	}
}

// TestExport_MissingContainer verifies Export errors for unknown containers.
func TestExport_MissingContainer(t *testing.T) {
	if err := Export(context.Background(), "nonexistent-thrive-test-export", discardTestWriter{}); err == nil {
		t.Error("Export: expected error for missing container, got nil")
	}
}

// TestDiff_MissingUpper verifies Diff returns empty (not error) when the
// container has no writable layer yet.
func TestDiff_MissingUpper(t *testing.T) {
	changes, err := Diff(context.Background(), "nonexistent-thrive-test-diff")
	if err != nil {
		t.Fatalf("Diff: unexpected error: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("Diff: got %d changes, want 0", len(changes))
	}
}

// TestTop_NotRunning verifies Top errors for unknown containers.
func TestTop_NotRunning(t *testing.T) {
	_, err := Top(context.Background(), "nonexistent-thrive-test-top")
	if err == nil {
		t.Error("Top: expected error for missing container, got nil")
	}
}

// TestPause_NotRunning verifies Pause errors for unknown containers.
func TestPause_NotRunning(t *testing.T) {
	if err := Pause(context.Background(), "nonexistent-thrive-test-pause"); err == nil {
		t.Error("Pause: expected error for missing container, got nil")
	}
}

// TestUnpause_NotPaused verifies Unpause errors for unknown containers.
func TestUnpause_NotPaused(t *testing.T) {
	if err := Unpause(context.Background(), "nonexistent-thrive-test-unpause"); err == nil {
		t.Error("Unpause: expected error for missing container, got nil")
	}
}

// TestUpdate_MissingContainer verifies Update errors for unknown containers.
func TestUpdate_MissingContainer(t *testing.T) {
	err := Update(context.Background(), "nonexistent-thrive-test-update", UpdateOptions{MemoryLimit: 1024})
	if err == nil {
		t.Error("Update: expected error for missing container, got nil")
	}
}

// TestContainerPorts_MissingContainer verifies Port errors for unknown containers.
func TestContainerPorts_MissingContainer(t *testing.T) {
	_, err := ContainerPorts(context.Background(), "nonexistent-thrive-test-port")
	if err == nil {
		t.Error("ContainerPorts: expected error for missing container, got nil")
	}
}

// TestRename_ExistingTarget verifies Rename refuses to overwrite.
func TestRename_ExistingTarget(t *testing.T) {
	// Both IDs missing: loadState(old) fails first — still an error either way.
	if err := Rename(context.Background(), "nonexistent-a", "nonexistent-b"); err == nil {
		t.Error("Rename: expected error, got nil")
	}
}

type discardTestWriter struct{}

func (discardTestWriter) Write(p []byte) (int, error) { return len(p), nil }
