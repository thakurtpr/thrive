//go:build linux
// +build linux

package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestSaveState_WritesStatusCreated verifies that saveState persists the
// ContainerState as valid JSON with the expected status field.
func TestSaveState_WritesStatusCreated(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	state := &ContainerState{
		ID:     "test-abc",
		Status: "created",
	}

	// Act
	if err := saveState(dir, state); err != nil {
		t.Fatalf("saveState: unexpected error: %v", err)
	}

	// Assert
	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("ReadFile state.json: %v", err)
	}

	var got ContainerState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal state.json: %v", err)
	}

	if got.Status != "created" {
		t.Errorf("status: got %q, want %q", got.Status, "created")
	}
	if got.ID != "test-abc" {
		t.Errorf("id: got %q, want %q", got.ID, "test-abc")
	}
}

// TestLoadState_ReturnsError_WhenMissing verifies that loadState returns an
// error when no state.json exists at the expected path.
func TestLoadState_ReturnsError_WhenMissing(t *testing.T) {
	// Arrange — "nonexistent-id" has no directory under /run/thrive/containers.
	// Act
	_, err := loadState("nonexistent-container-id-thrive-test")

	// Assert
	if err == nil {
		t.Fatal("loadState: expected error for missing state.json, got nil")
	}
}

// TestSaveAndLoadState_Roundtrip verifies that a state written by saveState
// is faithfully recovered by constructing the path that loadState would use,
// exercising the JSON serialisation end-to-end through unexported helpers.
func TestSaveAndLoadState_Roundtrip(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	want := &ContainerState{
		ID:       "rt-roundtrip",
		Status:   "running",
		PID:      12345,
		ExitCode: 0,
	}

	// Act
	if err := saveState(dir, want); err != nil {
		t.Fatalf("saveState: %v", err)
	}

	// Read back via raw file (loadState uses /run/thrive/... path, so we parse
	// the file directly to confirm saveState output is correct JSON).
	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var got ContainerState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// Assert
	if got.ID != want.ID {
		t.Errorf("ID: got %q, want %q", got.ID, want.ID)
	}
	if got.Status != want.Status {
		t.Errorf("Status: got %q, want %q", got.Status, want.Status)
	}
	if got.PID != want.PID {
		t.Errorf("PID: got %d, want %d", got.PID, want.PID)
	}
}
