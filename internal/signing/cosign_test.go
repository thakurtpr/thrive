package signing

import (
	"bytes"
	"context"
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
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
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

// annotatedImage injects per-layer annotations into the manifest (cosign
// stores each SimpleSigning base64 signature that way). remote.Write pushes
// via RawManifest, so the override lives there (a Manifest-only override
// would be bypassed on push).
type annotatedImage struct {
	v1.Image
	layerAnnotations map[string]string
}

func (a annotatedImage) RawManifest() ([]byte, error) {
	m, err := a.Image.Manifest()
	if err != nil {
		return nil, err
	}
	for i := range m.Layers {
		if m.Layers[i].Annotations == nil {
			m.Layers[i].Annotations = map[string]string{}
		}
		for k, v := range a.layerAnnotations {
			m.Layers[i].Annotations[k] = v
		}
	}
	return json.Marshal(m)
}

func (a annotatedImage) Manifest() (*v1.Manifest, error) {
	m, err := a.Image.Manifest()
	if err != nil {
		return nil, err
	}
	for i := range m.Layers {
		if m.Layers[i].Annotations == nil {
			m.Layers[i].Annotations = map[string]string{}
		}
		for k, v := range a.layerAnnotations {
			m.Layers[i].Annotations[k] = v
		}
	}
	return m, nil
}

// pushSigImage publishes a cosign-style .sig image for ref's digest to a
// fake registry: one SimpleSigning layer signed by signer, signature in the
// layer annotation. Returns the public key PEM path for --verify-key.
func pushSigImage(t *testing.T, srv string, imgDigest string, payload []byte, signer signature.Signer, pub crypto.PublicKey) string {
	t.Helper()
	sig, err := signer.SignMessage(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("SignMessage: %v", err)
	}
	layer := static.NewLayer(payload, types.MediaType("application/vnd.dev.cosign.simplesigning.v1+json"))
	base, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		t.Fatalf("AppendLayers: %v", err)
	}
	sigImg := annotatedImage{Image: base, layerAnnotations: map[string]string{
		"dev.cosignproject.cosign/signature": base64.StdEncoding.EncodeToString(sig),
	}}
	sigRef, err := name.ParseReference(srv + ":sha256-" + imgDigest + ".sig")
	if err != nil {
		t.Fatalf("sig ref: %v", err)
	}
	if err := remote.Write(sigRef, sigImg); err != nil {
		t.Fatalf("push .sig: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "cosign.pub")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	return keyPath
}

func simpleSigningPayload(imageDigest string) []byte {
	p, _ := json.Marshal(map[string]any{
		"critical": map[string]any{
			"type":   "cosign container image signature",
			"image":  map[string]any{"docker-manifest-digest": imageDigest},
			"expiry": "2099-01-01T00:00:00Z",
		},
		"optional": map[string]any{"test": true},
	})
	return p
}

// TestVerifyCosignImage_LiveRoundtrip exercises the full registry plumbing
// (Head → .sig ref → manifest → SimpleSigning layer → key check) against
// an in-memory fake registry. No network beyond loopback.
func TestVerifyCosignImage_LiveRoundtrip(t *testing.T) {
	srv := httptest.NewServer(registry.New())
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")

	ref, err := name.ParseReference(host + "/e2e:latest")
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(ref, empty.Image); err != nil {
		t.Fatalf("push test image: %v", err)
	}
	desc, err := remote.Head(ref)
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	digestHex := desc.Digest.Hex

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := signature.LoadSigner(priv, crypto.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	payload := simpleSigningPayload("sha256:" + digestHex)
	keyPath := pushSigImage(t, host+"/e2e", digestHex, payload, signer, &priv.PublicKey)

	ctx := context.Background()
	if err := VerifyCosignImage(ctx, host+"/e2e:latest", keyPath, "", ""); err != nil {
		t.Errorf("live roundtrip: %v", err)
	}
	// A different key must fail (proves the signature is actually checked,
	// not merely present).
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKIXPublicKey(&other.PublicKey)
	wrongKey := filepath.Join(t.TempDir(), "wrong.pub")
	if err := os.WriteFile(wrongKey, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyCosignImage(ctx, host+"/e2e:latest", wrongKey, "", ""); err == nil {
		t.Error("wrong key: expected error, got nil")
	}
}

// TestVerifyCosignImage_NoSignature verifies a clean error when the image
// has no .sig artifact at all.
func TestVerifyCosignImage_NoSignature(t *testing.T) {
	srv := httptest.NewServer(registry.New())
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")

	ref, err := name.ParseReference(host + "/unsigned:latest")
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(ref, empty.Image); err != nil {
		t.Fatalf("push: %v", err)
	}
	kp, err := sign.NewEphemeralKeypair(nil)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(kp.GetPublicKey())
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "cosign.pub")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyCosignImage(context.Background(), host+"/unsigned:latest", keyPath, "", ""); err == nil {
		t.Error("missing .sig: expected error, got nil")
	}
}
