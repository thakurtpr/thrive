package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/signing"
)

// isolatedHome redirects ~/.thrive (images + keys) to a temp dir so sign
// tests never touch the real home. Portable: covers $HOME (unix) and
// %USERPROFILE% (windows) resolution in os.UserHomeDir.
func isolatedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func writeManifest(t *testing.T, home, ref, digest string) {
	t.Helper()
	imgDir := filepath.Join(home, ".thrive", "images", image.SafeRef(ref))
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	meta, _ := json.Marshal(map[string]any{"Ref": ref, "Digest": digest})
	if err := os.WriteFile(filepath.Join(imgDir, "manifest.json"), meta, 0644); err != nil {
		t.Fatalf("fixture: %v", err)
	}
}

// TestRunSign_MissingImage verifies a clear error when nothing was pulled.
func TestRunSign_MissingImage(t *testing.T) {
	isolatedHome(t)
	err := runSign("never-pulled:99", "default")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("missing image: got %v, want not-found error", err)
	}
}

// TestRunSign_NoDigest verifies manifests without a digest are rejected.
func TestRunSign_NoDigest(t *testing.T) {
	home := isolatedHome(t)
	writeManifest(t, home, "nodigest:1", "")
	if err := runSign("nodigest:1", "default"); err == nil ||
		!strings.Contains(err.Error(), "no digest") {
		t.Errorf("empty digest: got %v, want no-digest error", err)
	}
}

// TestRunSign_Roundtrip signs a fake image (auto-generating the key),
// verifies the signature file, and signs again reusing the stored key.
func TestRunSign_Roundtrip(t *testing.T) {
	home := isolatedHome(t)
	ref, digest := "testimg:1", "sha256:abc123"
	writeManifest(t, home, ref, digest)
	if err := runSign(ref, "testkey"); err != nil {
		t.Fatalf("runSign: %v", err)
	}
	imgDir := filepath.Join(home, ".thrive", "images", image.SafeRef(ref))
	if err := signing.Verify(digest, signing.SigPath(imgDir, digest)); err != nil {
		t.Errorf("Verify after sign: %v", err)
	}
	// Second run must reuse the stored key, not fail or duplicate.
	if err := runSign(ref, "testkey"); err != nil {
		t.Errorf("runSign reuse key: %v", err)
	}
	if err := signing.Verify("sha256:wrong", signing.SigPath(imgDir, digest)); err == nil {
		t.Error("Verify wrong digest: expected error, got nil")
	}
}
