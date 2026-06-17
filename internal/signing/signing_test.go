package signing_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thakurprasadrout/thrive/internal/signing"
)

func TestGenerateAndLoadKeyPair(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	kp, err := signing.GenerateKeyPair("test")
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	if len(kp.PrivateKey) == 0 || len(kp.PublicKey) == 0 {
		t.Fatal("empty key pair returned")
	}
	loaded, err := signing.LoadKeyPair("test")
	if err != nil {
		t.Fatalf("LoadKeyPair: %v", err)
	}
	if string(loaded.PublicKey) != string(kp.PublicKey) {
		t.Fatal("loaded public key does not match generated")
	}
}

func TestSignAndVerify(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	kp, err := signing.GenerateKeyPair("test")
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	sigPath := filepath.Join(t.TempDir(), "img.sig")
	digest := "sha256:abc123def456abc123def456abc123def456abc123def456abc123def456abc1"

	sig, err := signing.Sign(digest, sigPath, kp)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if sig.ImageDigest != digest {
		t.Fatalf("wrong digest in sig: got %s", sig.ImageDigest)
	}
	if err := signing.Verify(digest, sigPath); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestVerify_WrongDigest(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	kp, _ := signing.GenerateKeyPair("test")
	sigPath := filepath.Join(t.TempDir(), "img.sig")
	digest := "sha256:abc123def456abc123def456abc123def456abc123def456abc123def456abc1"
	signing.Sign(digest, sigPath, kp)

	if err := signing.Verify("sha256:deadbeef", sigPath); err == nil {
		t.Fatal("expected verification failure for wrong digest")
	}
}

func TestVerify_MissingSigFile(t *testing.T) {
	if err := signing.Verify("sha256:abc", "/nonexistent/path.sig"); err == nil {
		t.Fatal("expected error for missing sig file")
	}
}

func TestSigPath_Deterministic(t *testing.T) {
	p1 := signing.SigPath("/tmp/images/alpine", "sha256:abc123")
	p2 := signing.SigPath("/tmp/images/alpine", "sha256:abc123")
	if p1 != p2 {
		t.Fatalf("SigPath not deterministic: %s != %s", p1, p2)
	}
	if filepath.Ext(p1) != ".sig" {
		t.Fatalf("SigPath should end in .sig, got %s", p1)
	}
}

func TestLoadKeyPair_Missing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := signing.LoadKeyPair("nonexistent"); err == nil {
		t.Fatal("expected error loading nonexistent key")
	}
}

func TestSign_CreatesSigFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	kp, _ := signing.GenerateKeyPair("test")
	sigPath := filepath.Join(t.TempDir(), "test.sig")
	_, err := signing.Sign("sha256:abc123", sigPath, kp)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if _, statErr := os.Stat(sigPath); os.IsNotExist(statErr) {
		t.Fatal("sig file not created")
	}
}
