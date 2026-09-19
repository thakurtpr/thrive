//go:build linux
// +build linux

package swarmconfig

import (
	"testing"
)

func testOverride(t *testing.T) {
	t.Helper()
	storeDirOverride = t.TempDir()
	t.Cleanup(func() { storeDirOverride = "" })
}

// TestCreateGetRemove verifies the config lifecycle.
func TestCreateGetRemove(t *testing.T) {
	testOverride(t)
	cfg, err := Create("app.conf", []byte("key=value\n"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if cfg.Size != 10 {
		t.Errorf("Size: got %d, want 10", cfg.Size)
	}
	data, err := Get("app.conf")
	if err != nil || string(data) != "key=value\n" {
		t.Errorf("Get: got %q, %v", data, err)
	}
	list, err := List()
	if err != nil || len(list) != 1 {
		t.Fatalf("List: got %v, %v", list, err)
	}
	if err := Remove("app.conf"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := Inspect("app.conf"); err == nil {
		t.Error("Inspect after remove: expected error, got nil")
	}
}

// TestCreate_Validation verifies name/data/dupe guards.
func TestCreate_Validation(t *testing.T) {
	testOverride(t)
	if _, err := Create("bad name!", []byte("x")); err == nil {
		t.Error("bad name: expected error, got nil")
	}
	if _, err := Create("empty", nil); err == nil {
		t.Error("empty data: expected error, got nil")
	}
	if _, err := Create("dup", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if _, err := Create("dup", []byte("x")); err == nil {
		t.Error("duplicate: expected error, got nil")
	}
	if _, err := Get("missing"); err == nil {
		t.Error("Get missing: expected error, got nil")
	}
	if err := Remove("missing"); err == nil {
		t.Error("Remove missing: expected error, got nil")
	}
}
