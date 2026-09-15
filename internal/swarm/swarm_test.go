//go:build linux
// +build linux

package swarm

import (
	"context"
	"testing"
)

func testOverride(t *testing.T) {
	t.Helper()
	swarmDirOverride = t.TempDir()
	t.Cleanup(func() { swarmDirOverride = "" })
}

// TestInitLeave verifies membership lifecycle without touching the host.
func TestInitLeave(t *testing.T) {
	testOverride(t)
	st, err := Init("10.0.0.1")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !st.Initialized || !st.Manager || st.WorkerToken == "" {
		t.Errorf("Init state: %+v", st)
	}
	if _, err := Init(""); err == nil {
		t.Error("Init twice: expected error, got nil")
	}
	got, err := Inspect()
	if err != nil || got.NodeID != st.NodeID {
		t.Errorf("Inspect: got %+v, %v", got, err)
	}
	if err := Leave(false); err != nil {
		t.Fatalf("Leave: %v", err)
	}
	if err := Leave(false); err == nil {
		t.Error("Leave twice: expected error, got nil")
	}
}

// TestJoinToken verifies token show/rotate.
func TestJoinToken(t *testing.T) {
	testOverride(t)
	if _, err := Init(""); err != nil {
		t.Fatal(err)
	}
	first, err := JoinToken("worker", false)
	if err != nil || first == "" {
		t.Fatalf("JoinToken: %v", first)
	}
	second, err := JoinToken("worker", true)
	if err != nil || second == first {
		t.Errorf("JoinToken rotate: got %q (was %q)", second, first)
	}
	if _, err := JoinToken("captain", false); err == nil {
		t.Error("JoinToken bad role: expected error, got nil")
	}
}

// TestJoin_Refused verifies remote clustering fails honestly.
func TestJoin_Refused(t *testing.T) {
	if err := Join("10.0.0.2", "THRIVE-worker-abc"); err == nil {
		t.Error("Join: expected error, got nil")
	}
}

// TestServiceValidation verifies create-time validation without runtime.
func TestServiceValidation(t *testing.T) {
	testOverride(t)
	ctx := context.Background()
	if _, err := CreateService(ctx, "web", ServiceSpec{Image: "nginx"}); err == nil {
		t.Error("CreateService uninitialised: expected error, got nil")
	}
	if _, err := Init(""); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateService(ctx, "bad name!", ServiceSpec{Image: "nginx"}); err == nil {
		t.Error("CreateService bad name: expected error, got nil")
	}
	if _, err := CreateService(ctx, "web", ServiceSpec{}); err == nil {
		t.Error("CreateService no image: expected error, got nil")
	}
	if err := ScaleService(ctx, "web", -1); err == nil {
		t.Error("ScaleService negative: expected error, got nil")
	}
	if err := UpdateService(ctx, "web", ""); err == nil {
		t.Error("UpdateService empty image: expected error, got nil")
	}
	if err := RollbackService(ctx, "web"); err == nil {
		t.Error("RollbackService missing: expected error, got nil")
	}
	if err := RemoveService(ctx, "web"); err == nil {
		t.Error("RemoveService missing: expected error, got nil")
	}
}

// TestRequireInitialised verifies the gate.
func TestRequireInitialised(t *testing.T) {
	testOverride(t)
	if err := RequireInitialised(); err == nil {
		t.Error("RequireInitialised: expected error, got nil")
	}
	if _, err := Init(""); err != nil {
		t.Fatal(err)
	}
	if err := RequireInitialised(); err != nil {
		t.Errorf("RequireInitialised: %v", err)
	}
}
