package signing

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sigstore/sigstore-go/pkg/sign"
	"github.com/sigstore/sigstore/pkg/signature"
	"google.golang.org/protobuf/encoding/protojson"
)

// TestCosignBundle_Roundtrip signs data with an ephemeral key via
// sigstore-go and verifies it through verifyCosignBundle (offline).
func TestCosignBundle_Roundtrip(t *testing.T) {
	kp, err := sign.NewEphemeralKeypair(nil)
	if err != nil {
		t.Fatalf("NewEphemeralKeypair: %v", err)
	}
	data := []byte("thrive cosign test artifact")
	sum := sha256.Sum256(data)
	pb, err := sign.Bundle(&sign.PlainData{Data: data}, kp, sign.BundleOptions{})
	if err != nil {
		t.Fatalf("sign.Bundle: %v", err)
	}
	raw, err := protojson.Marshal(pb)
	if err != nil {
		t.Fatalf("marshal bundle: %v", err)
	}
	verifier, err := signature.LoadVerifier(kp.GetPublicKey(), crypto.SHA256)
	if err != nil {
		t.Fatalf("LoadVerifier: %v", err)
	}
	if err := verifyCosignBundle(raw, hex.EncodeToString(sum[:]), verifier); err != nil {
		t.Errorf("verifyCosignBundle: %v", err)
	}
}

// TestCosignBundle_WrongDigest verifies a digest mismatch fails.
func TestCosignBundle_WrongDigest(t *testing.T) {
	kp, err := sign.NewEphemeralKeypair(nil)
	if err != nil {
		t.Fatalf("NewEphemeralKeypair: %v", err)
	}
	data := []byte("thrive cosign test artifact")
	pb, err := sign.Bundle(&sign.PlainData{Data: data}, kp, sign.BundleOptions{})
	if err != nil {
		t.Fatalf("sign.Bundle: %v", err)
	}
	raw, err := protojson.Marshal(pb)
	if err != nil {
		t.Fatalf("marshal bundle: %v", err)
	}
	verifier, err := signature.LoadVerifier(kp.GetPublicKey(), crypto.SHA256)
	if err != nil {
		t.Fatalf("LoadVerifier: %v", err)
	}
	other := sha256.Sum256([]byte("something else"))
	if err := verifyCosignBundle(raw, hex.EncodeToString(other[:]), verifier); err == nil {
		t.Error("verifyCosignBundle with wrong digest: expected error, got nil")
	}
}

// TestSimpleSigning_Roundtrip signs a SimpleSigning payload with ECDSA and
// verifies it through verifySimpleSigning (offline).
func TestSimpleSigning_Roundtrip(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	signer, err := signature.LoadSigner(priv, crypto.SHA256)
	if err != nil {
		t.Fatalf("LoadSigner: %v", err)
	}
	verifier, err := signature.LoadVerifier(&priv.PublicKey, crypto.SHA256)
	if err != nil {
		t.Fatalf("LoadVerifier: %v", err)
	}
	payload, _ := json.Marshal(map[string]any{
		"critical": map[string]any{"type": "cosign container image signature"},
	})
	sig, err := signer.SignMessage(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("SignMessage: %v", err)
	}
	sigB64 := base64.StdEncoding.EncodeToString(sig)
	if err := verifySimpleSigning(payload, sigB64, verifier); err != nil {
		t.Errorf("verifySimpleSigning: %v", err)
	}
	if err := verifySimpleSigning(append(payload, 'x'), sigB64, verifier); err == nil {
		t.Error("verifySimpleSigning tampered payload: expected error, got nil")
	}
	if err := verifySimpleSigning(payload, "", verifier); err == nil {
		t.Error("verifySimpleSigning empty sig: expected error, got nil")
	}
}

// TestLoadCosignVerifier_BadInputs covers missing files and invalid PEM.
func TestLoadCosignVerifier_BadInputs(t *testing.T) {
	if _, err := loadCosignVerifier("/nonexistent/cosign.pub"); err == nil {
		t.Error("missing key file: expected error, got nil")
	}
	bad := filepath.Join(t.TempDir(), "bad.pub")
	if err := os.WriteFile(bad, []byte("not pem"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCosignVerifier(bad); err == nil {
		t.Error("invalid PEM: expected error, got nil")
	}
	derBad := filepath.Join(t.TempDir(), "der.pub")
	if err := os.WriteFile(derBad, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte{1, 2, 3}}), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCosignVerifier(derBad); err == nil {
		t.Error("invalid DER: expected error, got nil")
	}
}

// TestVerifyCosignImage_BadKey verifies a missing key file fails before any
// network access (key load is first).
func TestVerifyCosignImage_BadKey(t *testing.T) {
	err := VerifyCosignImage(t.Context(), "alpine:latest", "/nonexistent/cosign.pub", "", "")
	if err == nil || !strings.Contains(err.Error(), "read key") {
		t.Errorf("missing key: got %v, want read-key error", err)
	}
}

// TestVerifyCosignImage_BadRef verifies an unparseable ref fails after a
// valid key loads (still no network).
func TestVerifyCosignImage_BadRef(t *testing.T) {
	kp, err := sign.NewEphemeralKeypair(nil)
	if err != nil {
		t.Fatalf("NewEphemeralKeypair: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(kp.GetPublicKey())
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "cosign.pub")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	if err := os.WriteFile(keyPath, pemBytes, 0600); err != nil {
		t.Fatal(err)
	}
	err = VerifyCosignImage(t.Context(), ":::not-a-ref", keyPath, "", "")
	if err == nil || !strings.Contains(err.Error(), "parse reference") {
		t.Errorf("bad ref: got %v, want parse-reference error", err)
	}
}
