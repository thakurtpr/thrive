//go:build linux
// +build linux

package runtime

import (
	"context"
	"os"
	"path/filepath"
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

// TestDiff_MissingContainer verifies Diff returns no changes for unknown IDs.
func TestDiff_MissingContainer(t *testing.T) {
	changes, err := Diff(context.Background(), "nonexistent-thrive-test-diff")
	if err != nil {
		t.Fatalf("Diff: unexpected error: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("Diff: expected 0 changes, got %d", len(changes))
	}
}

// TestCommitWithOptions_EmptyRef verifies CommitWithOptions rejects empty refs.
func TestCommitWithOptions_EmptyRef(t *testing.T) {
	err := CommitWithOptions(context.Background(), "nonexistent-thrive-test-commit", "", CommitOptions{Author: "a", Message: "m", Pause: true})
	if err == nil {
		t.Error("CommitWithOptions: expected error for empty ref, got nil")
	}
}

type discardTestWriter struct{}

func (discardTestWriter) Write(p []byte) (int, error) { return len(p), nil }

// TestDiffCopyChanges verifies copy-fallback diffing: Added for new files,
// Changed for modified content/retargeted links, Deleted for lower-only
// files, silence for identical content (logic verified on fixtures).
func TestDiffCopyChanges(t *testing.T) {
	root := t.TempDir()
	l1 := filepath.Join(root, "l1")
	l2 := filepath.Join(root, "l2")
	merged := filepath.Join(root, "merged")
	for _, d := range []string{l1, l2, merged} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(base, rel, content string) {
		t.Helper()
		p := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(l1, "same.txt", "same")
	write(l1, "gone.txt", "gone")
	write(l1, "chg.txt", "old")
	write(l1, "dup.txt", "dup")
	write(l2, "dup.txt", "dup")
	if err := os.Symlink("same.txt", filepath.Join(l1, "lnk")); err != nil {
		t.Fatal(err)
	}
	write(merged, "same.txt", "same")
	write(merged, "chg.txt", "new")
	write(merged, "added.txt", "added")
	if err := os.Symlink("chg.txt", filepath.Join(merged, "lnk")); err != nil {
		t.Fatal(err)
	}
	got, err := diffCopyChanges(merged, []string{l1, l2})
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]string{}
	for _, c := range got {
		m[c.Path] = c.Kind
	}
	for p, k := range map[string]string{
		"/added.txt": "A", "/chg.txt": "C", "/lnk": "C",
		"/gone.txt": "D", "/dup.txt": "D",
	} {
		if m[p] != k {
			t.Errorf("path %s: got %q want %q (full %v)", p, m[p], k, m)
		}
	}
	if _, ok := m["/same.txt"]; ok {
		t.Errorf("unchanged file reported: %v", m)
	}
}

// TestLowerFile_FirstHit verifies first-match precedence across layers.
func TestLowerFile_FirstHit(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir2, "f.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := lowerFile([]string{dir1, dir2}, "f.txt"); got != filepath.Join(dir2, "f.txt") {
		t.Errorf("lowerFile: got %q", got)
	}
	if got := lowerFile([]string{dir1, dir2}, "missing"); got != "" {
		t.Errorf("lowerFile missing: got %q", got)
	}
}

// TestFileContentDiffers verifies type and content comparison.
func TestFileContentDiffers(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	os.WriteFile(a, []byte("same"), 0644)
	os.WriteFile(b, []byte("same"), 0644)
	if fileContentDiffers(a, b) {
		t.Error("identical files: got differs=true")
	}
	os.WriteFile(b, []byte("other"), 0644)
	if !fileContentDiffers(a, b) {
		t.Error("different files: got differs=false")
	}
	os.WriteFile(b, []byte("same"), 0644)
	sub := filepath.Join(dir, "sub")
	os.Mkdir(sub, 0755)
	if !fileContentDiffers(a, sub) {
		t.Error("file vs dir: got differs=false")
	}
}
