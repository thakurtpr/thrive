package contextstore

import (
	"path/filepath"
	"testing"
)

func testOverride(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	contextsDirOverride = dir
	currentFileOverride = filepath.Join(dir, "current")
	t.Cleanup(func() { contextsDirOverride = ""; currentFileOverride = "" })
}

// TestContextCRUD verifies context lifecycle.
func TestContextCRUD(t *testing.T) {
	testOverride(t)
	c, err := Create("prod", "Production", "tcp://10.0.0.5:2375")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.Endpoint != "tcp://10.0.0.5:2375" {
		t.Errorf("Endpoint: got %q", c.Endpoint)
	}
	if _, err := Create("prod", "", ""); err == nil {
		t.Error("Create duplicate: expected error, got nil")
	}
	if _, err := Create("bad name!", "", ""); err == nil {
		t.Error("Create invalid: expected error, got nil")
	}
	// Default endpoint fallback.
	d, err := Create("local", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if d.Endpoint != DefaultEndpoint {
		t.Errorf("Endpoint: got %q, want default", d.Endpoint)
	}
	if err := Use("prod"); err != nil {
		t.Fatalf("Use: %v", err)
	}
	if Current() != "prod" {
		t.Errorf("Current: got %q", Current())
	}
	if err := Use("missing"); err == nil {
		t.Error("Use missing: expected error, got nil")
	}
	list, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// default + prod + local
	if len(list) != 3 {
		t.Errorf("List: got %d contexts, want 3", len(list))
	}
	if err := Remove("default"); err == nil {
		t.Error("Remove default: expected error, got nil")
	}
	if err := Remove("prod"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if Current() != "default" {
		t.Errorf("Current after removing active: got %q", Current())
	}
}
