//go:build linux
// +build linux

package runtime

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
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

// TestParseSignal verifies docker-style signal mapping shared by the Linux
// compose kill path and the VM daemon (Linux CI).
func TestParseSignal(t *testing.T) {
	cases := map[string]syscall.Signal{
		"": syscall.SIGKILL, "KILL": syscall.SIGKILL,
		"TERM": syscall.SIGTERM, "9": syscall.Signal(9),
		"15": syscall.Signal(15), "BOGUS": syscall.SIGKILL,
	}
	for in, want := range cases {
		if got := ParseSignal(in); got != want {
			t.Errorf("ParseSignal(%q): got %v want %v", in, got, want)
		}
	}
}

// TestProcessAlive verifies liveness detection incl. zombies and the
// reaped (reparented-then-gone) case behind state reconciliation.
func TestProcessAlive(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Error("own pid: got dead, want alive")
	}
	if processAlive(1 << 20) {
		t.Error("bogus pid: got alive, want dead")
	}
	// Exited, never-waited child: zombie (or reaped) → not alive.
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot fork: %v", err)
	}
	zombie := cmd.Process.Pid
	dead := false
	for i := 0; i < 200; i++ {
		if !processAlive(zombie) {
			dead = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cmd.Wait() //nolint:errcheck
	if !dead {
		t.Errorf("exited child pid %d: got alive, want dead", zombie)
	}
	// Reaped child: /proc entry gone → not alive.
	if processAlive(zombie) {
		t.Errorf("reaped pid %d: got alive, want dead", zombie)
	}
}

// TestReconcileState_FlipsDead verifies orphaned running states become
// stopped without touching live or non-running states.
func TestReconcileState_FlipsDead(t *testing.T) {
	dir := t.TempDir()
	dead := &ContainerState{ID: "x", Status: "running", PID: 1 << 20}
	if got := reconcileState(dir, dead); got.Status != "stopped" || got.ExitCode != -1 {
		t.Errorf("dead container: got %+v", got)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "state.json")); err != nil {
		t.Errorf("reconciled state not persisted: %v", err)
	} else {
		var persisted ContainerState
		if err := json.Unmarshal(data, &persisted); err != nil || persisted.Status != "stopped" {
			t.Errorf("persisted state: got %s, %v", string(data), err)
		}
	}
	live := &ContainerState{ID: "x", Status: "running", PID: os.Getpid()}
	if got := reconcileState(dir, live); got.Status != "running" {
		t.Errorf("live container: got %+v", got)
	}
	stopped := &ContainerState{ID: "x", Status: "stopped", PID: 1 << 20}
	if got := reconcileState(dir, stopped); got.Status != "stopped" || got.ExitCode != 0 {
		t.Errorf("stopped container touched: got %+v", got)
	}
}
