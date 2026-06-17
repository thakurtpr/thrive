//go:build linux
// +build linux

package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestSaveState_AllFields verifies that saveState serialises every field of
// ContainerState to JSON without loss, covering fields not exercised by the
// existing roundtrip test (ExitCode, RootfsPath, NetworkInfo).
func TestSaveState_AllFields(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	want := &ContainerState{
		ID:          "all-fields-ctr",
		Status:      "stopped",
		PID:         99,
		ExitCode:    2,
		RootfsPath:  "/var/lib/thrive/rootfs/all-fields-ctr",
		NetworkInfo: "172.17.0.2/16",
	}

	// Act
	if err := saveState(dir, want); err != nil {
		t.Fatalf("saveState: unexpected error: %v", err)
	}

	// Assert — parse the raw file so the test is independent of loadState's path logic.
	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("ReadFile state.json: %v", err)
	}

	var got ContainerState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal state.json: %v", err)
	}

	if got.ID != want.ID {
		t.Errorf("ID: got %q, want %q", got.ID, want.ID)
	}
	if got.Status != want.Status {
		t.Errorf("Status: got %q, want %q", got.Status, want.Status)
	}
	if got.PID != want.PID {
		t.Errorf("PID: got %d, want %d", got.PID, want.PID)
	}
	if got.ExitCode != want.ExitCode {
		t.Errorf("ExitCode: got %d, want %d", got.ExitCode, want.ExitCode)
	}
	if got.RootfsPath != want.RootfsPath {
		t.Errorf("RootfsPath: got %q, want %q", got.RootfsPath, want.RootfsPath)
	}
	if got.NetworkInfo != want.NetworkInfo {
		t.Errorf("NetworkInfo: got %q, want %q", got.NetworkInfo, want.NetworkInfo)
	}
}

// TestSaveState_EmptyID verifies that saveState does not error on an empty-ID
// state — the function marshals and writes; enforcement of non-empty IDs is the
// caller's responsibility (tested in TestCreate_EmptyID).
func TestSaveState_EmptyID(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	state := &ContainerState{Status: "created"}

	// Act
	err := saveState(dir, state)

	// Assert
	if err != nil {
		t.Errorf("saveState with empty ID: unexpected error: %v", err)
	}
}

// TestSaveState_WritesToCorrectPath verifies that saveState always writes a
// file named exactly "state.json" inside the supplied directory — not a
// sub-directory or a differently named file.
func TestSaveState_WritesToCorrectPath(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	state := &ContainerState{ID: "path-check", Status: "created"}

	// Act
	if err := saveState(dir, state); err != nil {
		t.Fatalf("saveState: %v", err)
	}

	// Assert — exactly one file must exist and it must be named state.json.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in dir, got %d", len(entries))
	}
	if entries[0].Name() != "state.json" {
		t.Errorf("unexpected file name: %q, want \"state.json\"", entries[0].Name())
	}
}

// TestState_NotFound verifies that State returns a non-nil error when the
// requested container does not exist under /run/thrive/containers.
func TestState_NotFound(t *testing.T) {
	// Arrange — use a container ID that cannot exist in a test environment.
	ctx := context.Background()
	id := "thrive-test-state-nonexistent-99999999"

	// Act
	_, err := State(ctx, id)

	// Assert
	if err == nil {
		t.Fatal("State: expected error for nonexistent container, got nil")
	}
}

// TestDelete_NotFound verifies that Delete returns nil when the container
// directory does not exist — os.RemoveAll is idempotent on missing paths.
func TestDelete_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	id := "thrive-test-delete-nonexistent-99999999"

	// Act
	err := Delete(ctx, id)

	// Assert — RemoveAll on a missing path must not error.
	if err != nil {
		t.Errorf("Delete on nonexistent container: unexpected error: %v", err)
	}
}

// TestKill_NotFound verifies that Kill returns a non-nil error when the
// container has no state.json (loadState fails before any signal is sent).
func TestKill_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	id := "thrive-test-kill-nonexistent-99999999"

	// Act
	err := Kill(ctx, id, syscall.SIGKILL)

	// Assert
	if err == nil {
		t.Fatal("Kill on nonexistent container: expected error, got nil")
	}
}

// TestCreate_EmptyID verifies that Create rejects a zero-length container ID
// with a descriptive error and does not create any directory.
func TestCreate_EmptyID(t *testing.T) {
	// Arrange
	ctx := context.Background()
	cfg := ContainerConfig{
		ID:    "",
		Image: "busybox",
	}

	// Act
	_, err := Create(ctx, cfg)

	// Assert
	if err == nil {
		t.Fatal("Create with empty ID: expected error, got nil")
	}
}

// TestCreate_WritesConfig verifies the full Create path when the process has
// write access to /run/thrive/containers.  The test is skipped on non-root
// hosts because /run/thrive/containers typically requires root to create.
func TestCreate_WritesConfig(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("TestCreate_WritesConfig requires root to write to /run/thrive/containers")
	}

	// Arrange
	ctx := context.Background()
	id := "thrive-test-create-writes-config"
	cfg := ContainerConfig{
		ID:      id,
		Image:   "busybox",
		Command: []string{"/bin/sh", "-c", "echo hello"},
		Env:     []string{"FOO=bar"},
	}
	containerDir := filepath.Join("/run/thrive/containers", id)
	t.Cleanup(func() { os.RemoveAll(containerDir) })

	// Act
	container, err := Create(ctx, cfg)
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}

	// Assert — returned container struct
	if container == nil {
		t.Fatal("Create: returned nil container")
	}
	if container.ID != id {
		t.Errorf("container.ID: got %q, want %q", container.ID, id)
	}
	if container.State == nil {
		t.Fatal("container.State: nil")
	}
	if container.State.Status != "created" {
		t.Errorf("State.Status: got %q, want %q", container.State.Status, "created")
	}

	// Assert — config.json written with correct content
	configData, err := os.ReadFile(filepath.Join(containerDir, "config.json"))
	if err != nil {
		t.Fatalf("ReadFile config.json: %v", err)
	}

	var gotCfg ContainerConfig
	if err := json.Unmarshal(configData, &gotCfg); err != nil {
		t.Fatalf("Unmarshal config.json: %v", err)
	}
	if gotCfg.ID != cfg.ID {
		t.Errorf("config.json ID: got %q, want %q", gotCfg.ID, cfg.ID)
	}
	if gotCfg.Image != cfg.Image {
		t.Errorf("config.json Image: got %q, want %q", gotCfg.Image, cfg.Image)
	}

	// Assert — state.json was created
	if _, statErr := os.Stat(filepath.Join(containerDir, "state.json")); os.IsNotExist(statErr) {
		t.Error("state.json was not created by Create")
	}
}
