package buildx

import (
	"os"
	"path/filepath"
	"testing"
)

func testOverride(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	buildersDirOverride = dir
	currentFileOverride = filepath.Join(dir, "current")
	t.Cleanup(func() { buildersDirOverride = ""; currentFileOverride = "" })
}

// TestBuilderCRUD verifies builder lifecycle.
func TestBuilderCRUD(t *testing.T) {
	testOverride(t)
	b, err := CreateBuilder("dev", "", "")
	if err != nil {
		t.Fatalf("CreateBuilder: %v", err)
	}
	if b.Driver != "thrive" {
		t.Errorf("Driver: got %q, want thrive", b.Driver)
	}
	if _, err := CreateBuilder("dev", "", ""); err == nil {
		t.Error("CreateBuilder duplicate: expected error, got nil")
	}
	if _, err := CreateBuilder("bad driver!", "", ""); err == nil {
		t.Error("CreateBuilder invalid name: expected error, got nil")
	}
	got, err := InspectBuilder("dev")
	if err != nil {
		t.Fatalf("InspectBuilder: %v", err)
	}
	if !got.Current {
		t.Error("first builder should be current")
	}
	if err := UseBuilder("dev"); err != nil {
		t.Fatalf("UseBuilder: %v", err)
	}
	if CurrentBuilder() != "dev" {
		t.Errorf("CurrentBuilder: got %q", CurrentBuilder())
	}
	list, err := ListBuilders()
	if err != nil || len(list) != 1 {
		t.Fatalf("ListBuilders: got %v, %v", list, err)
	}
	if err := RemoveBuilder("dev"); err != nil {
		t.Fatalf("RemoveBuilder: %v", err)
	}
	if err := RemoveBuilder("dev"); err == nil {
		t.Error("RemoveBuilder missing: expected error, got nil")
	}
}

// TestCacheUsagePrune verifies cache accounting on a fixture dir.
func TestCacheUsagePrune(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a"), []byte("12345"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b"), []byte("123"), 0644); err != nil {
		t.Fatal(err)
	}

	if got := CacheUsage(dir); got != 8 {
		t.Errorf("CacheUsage: got %d, want 8", got)
	}
	if got := CacheUsage(filepath.Join(dir, "missing")); got != 0 {
		t.Errorf("CacheUsage missing: got %d, want 0", got)
	}
	reclaimed, err := PruneCache(dir)
	if err != nil {
		t.Fatalf("PruneCache: %v", err)
	}
	if reclaimed != 8 {
		t.Errorf("PruneCache: got %d, want 8", reclaimed)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("PruneCache: dir not empty: %v", entries)
	}
}
