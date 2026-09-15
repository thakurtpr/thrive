package volume

import (
	"os"
	"path/filepath"
	"testing"
)

func testOverride(t *testing.T) {
	t.Helper()
	volumeDirOverride = t.TempDir()
	t.Cleanup(func() { volumeDirOverride = "" })
}

// TestCreateInspect_Roundtrip verifies volume creation and retrieval.
func TestCreateInspect_Roundtrip(t *testing.T) {
	testOverride(t)
	v, err := Create("webdata")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if v.Name != "webdata" {
		t.Errorf("Name: got %q", v.Name)
	}
	if _, err := os.Stat(filepath.Join(v.Path)); err != nil {
		t.Errorf("volume data dir missing: %v", err)
	}
	got, err := Inspect("webdata")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if got.Path != v.Path {
		t.Errorf("Path: got %q, want %q", got.Path, v.Path)
	}
}

// TestCreate_Duplicate verifies creating the same volume twice fails.
func TestCreate_Duplicate(t *testing.T) {
	testOverride(t)
	if _, err := Create("dup"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := Create("dup"); err == nil {
		t.Error("Create duplicate: expected error, got nil")
	}
}

// TestCreate_InvalidName verifies name validation.
func TestCreate_InvalidName(t *testing.T) {
	testOverride(t)
	for _, bad := range []string{"", "../escape", "a/b", ".", "..", "has space"} {
		if _, err := Create(bad); err == nil {
			t.Errorf("Create(%q): expected error, got nil", bad)
		}
	}
}

// TestEnsure_Idempotent verifies Ensure creates once and returns after.
func TestEnsure_Idempotent(t *testing.T) {
	testOverride(t)
	first, err := Ensure("cache")
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	second, err := Ensure("cache")
	if err != nil {
		t.Fatalf("Ensure again: %v", err)
	}
	if first.Path != second.Path {
		t.Errorf("Ensure paths differ: %q vs %q", first.Path, second.Path)
	}
}

// TestList verifies listing returns created volumes.
func TestList(t *testing.T) {
	testOverride(t)
	if _, err := Create("one"); err != nil {
		t.Fatal(err)
	}
	if _, err := Create("two"); err != nil {
		t.Fatal(err)
	}
	vols, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(vols) != 2 {
		t.Errorf("List: got %d volumes, want 2", len(vols))
	}
}

// TestRemove verifies deletion and missing-volume errors.
func TestRemove(t *testing.T) {
	testOverride(t)
	if _, err := Create("gone"); err != nil {
		t.Fatal(err)
	}
	if err := Remove("gone", false, nil); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := Inspect("gone"); err == nil {
		t.Error("Inspect after remove: expected error, got nil")
	}
	if err := Remove("missing", false, nil); err == nil {
		t.Error("Remove missing: expected error, got nil")
	}
}

// TestRemove_InUse verifies the in-use guard and --force override.
func TestRemove_InUse(t *testing.T) {
	testOverride(t)
	v, err := Create("busy")
	if err != nil {
		t.Fatal(err)
	}
	inUse := func(path string) bool { return path == v.Path }
	if err := Remove("busy", false, inUse); err == nil {
		t.Error("Remove in-use: expected error, got nil")
	}
	if err := Remove("busy", true, inUse); err != nil {
		t.Errorf("Remove --force: %v", err)
	}
}

// TestPrune verifies only unused volumes are removed.
func TestPrune(t *testing.T) {
	testOverride(t)
	if _, err := Create("keep"); err != nil {
		t.Fatal(err)
	}
	used, err := Create("used")
	if err != nil {
		t.Fatal(err)
	}
	removed, err := Prune(func(path string) bool { return path == used.Path })
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(removed) != 1 || removed[0] != "keep" {
		t.Errorf("Prune: got %v, want [keep]", removed)
	}
}

// TestIsNamedVolume verifies bind-vs-named classification.
func TestIsNamedVolume(t *testing.T) {
	for src, want := range map[string]bool{
		"webdata":    true,
		"my-vol_1.2": true,
		"/host/path": false,
		"./rel":      false,
		".":          false,
		"has space":  false,
		"":           false,
	} {
		if got := IsNamedVolume(src); got != want {
			t.Errorf("IsNamedVolume(%q): got %v, want %v", src, got, want)
		}
	}
}
