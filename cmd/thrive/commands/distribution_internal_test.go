package commands

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/thakurprasadrout/thrive/internal/image"
)

// stubCosign replaces cosignVerifyFunc for a test, restoring afterwards.
func stubCosign(t *testing.T, fn func(ctx context.Context, ref, key, user, pass string) error) {
	t.Helper()
	prev := cosignVerifyFunc
	cosignVerifyFunc = fn
	t.Cleanup(func() { cosignVerifyFunc = prev })
}

// TestVerifyPulledImage_NoVerify verifies the pass-through path reports
// success without touching the registry.
func TestVerifyPulledImage_NoVerify(t *testing.T) {
	stubCosign(t, func(ctx context.Context, ref, key, user, pass string) error {
		return errors.New("must not be called")
	})
	ctx := context.Background()
	if !verifyPulledImage(ctx, "alpine", "alpine", "sha256:abc", "", "", false, "", false) {
		t.Error("no-verify: got false, want true")
	}
	if !verifyPulledImage(ctx, "alpine", "alpine", "sha256:abc", "", "", false, "", true) {
		t.Error("no-verify quiet: got false, want true")
	}
}

// TestVerifyPulledImage_Success verifies a passing cosign check.
func TestVerifyPulledImage_Success(t *testing.T) {
	var gotRef, gotKey, gotUser, gotPass string
	stubCosign(t, func(ctx context.Context, ref, key, user, pass string) error {
		gotRef, gotKey, gotUser, gotPass = ref, key, user, pass
		return nil
	})
	if !verifyPulledImage(context.Background(), "reg/img:tag", "reg/img:tag", "sha256:abc", "u", "p", true, "/k.pub", false) {
		t.Fatal("verify success: got false, want true")
	}
	if gotRef != "reg/img:tag" || gotKey != "/k.pub" || gotUser != "u" || gotPass != "p" {
		t.Errorf("verify args: got %q %q %q %q", gotRef, gotKey, gotUser, gotPass)
	}
}

// TestVerifyPulledImage_FailureRemovesImage verifies a failed check
// returns false and removes the just-pulled image from the host store.
func TestVerifyPulledImage_FailureRemovesImage(t *testing.T) {
	stubCosign(t, func(ctx context.Context, ref, key, user, pass string) error {
		return errors.New("bad signature")
	})
	pulledRef := "thrive-test-untrusted-image"
	imgDir := filepath.Join(image.StoreDir(), image.SafeRef(pulledRef))
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		t.Skipf("skipping: image store not writable (%v)", err)
	}
	t.Cleanup(func() { os.RemoveAll(imgDir) })
	if verifyPulledImage(context.Background(), pulledRef, pulledRef, "sha256:abc", "", "", true, "/k.pub", false) {
		t.Error("verify failure: got true, want false")
	}
	if _, err := os.Stat(imgDir); !os.IsNotExist(err) {
		t.Errorf("untrusted image left behind at %s", imgDir)
	}
}
