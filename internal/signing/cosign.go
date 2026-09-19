// Cosign/Sigstore image verification (key-based) via sigstore-go.
//
// Thrive stays thrive-native for its own `sign`/`verify` commands (Ed25519).
// This file adds cosign interoperability: `thrive pull --verify --verify-key
// cosign.pub` checks the OCI signature artifact cosign publishes alongside
// the image (tag `sha256-<digest>.sig`), accepting either a Sigstore bundle
// (verified through sigstore-go) or a legacy SimpleSigning payload (verified
// with the same key through sigstore/sigstore).
//
// Keyless (Fulcio/Rekor) identities are out of scope: verification is
// offline against an explicit public key file.
package signing

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/sigstore/sigstore/pkg/signature"
	"google.golang.org/protobuf/encoding/protojson"
)

// cosignSignatureAnnotation is where legacy cosign stores the base64
// signature for each SimpleSigning layer in the .sig image manifest.
const cosignSignatureAnnotation = "dev.cosignproject.cosign/signature"

// cosignBundleMediaFragment identifies Sigstore bundle layers.
const cosignBundleMediaFragment = "sigstore.bundle"

// VerifyCosignImage verifies that the OCI image ref carries a cosign
// signature valid under the PEM-encoded public key at pubKeyPath.
// Registry credentials are optional (anonymous when empty).
func VerifyCosignImage(ctx context.Context, ref, pubKeyPath, username, password string) error {
	verifier, err := loadCosignVerifier(pubKeyPath)
	if err != nil {
		return err
	}
	parsed, err := name.ParseReference(ref)
	if err != nil {
		return fmt.Errorf("signing: cosign: parse reference: %w", err)
	}
	var auth authn.Authenticator
	if username != "" {
		auth = &authn.Basic{Username: username, Password: password}
	}
	desc, err := remote.Head(parsed, remote.WithAuth(auth), remote.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("signing: cosign: resolve %s: %w", ref, err)
	}
	digest := desc.Digest
	if digest.Algorithm != "sha256" {
		return fmt.Errorf("signing: cosign: unsupported digest algorithm %q", digest.Algorithm)
	}
	sigRef, err := name.ParseReference(parsed.Context().Name() + ":sha256-" + digest.Hex + ".sig")
	if err != nil {
		return fmt.Errorf("signing: cosign: signature ref: %w", err)
	}
	sigImg, err := remote.Image(sigRef, remote.WithAuth(auth), remote.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("signing: cosign: no signature found for %s (looked for %s): %w", ref, sigRef.Name(), err)
	}
	manifest, err := sigImg.Manifest()
	if err != nil {
		return fmt.Errorf("signing: cosign: read signature manifest: %w", err)
	}
	layers, err := sigImg.Layers()
	if err != nil {
		return fmt.Errorf("signing: cosign: read signature layers: %w", err)
	}
	var failures []string
	for i, layer := range layers {
		mt, err := layer.MediaType()
		if err != nil {
			continue
		}
		rc, err := layer.Uncompressed()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}
		if strings.Contains(string(mt), cosignBundleMediaFragment) {
			if err := verifyCosignBundle(data, digest.Hex, verifier); err != nil {
				failures = append(failures, fmt.Sprintf("layer %d bundle: %v", i, err))
				continue
			}
			return nil
		}
		if looksLikeSimpleSigning(data) {
			sigB64 := ""
			if i < len(manifest.Layers) {
				sigB64 = manifest.Layers[i].Annotations[cosignSignatureAnnotation]
			}
			if err := verifySimpleSigning(data, sigB64, verifier); err != nil {
				failures = append(failures, fmt.Sprintf("layer %d simplesigning: %v", i, err))
				continue
			}
			return nil
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("signing: cosign: verification FAILED for %s: %s", ref, strings.Join(failures, "; "))
	}
	return fmt.Errorf("signing: cosign: no usable signature layer found for %s", ref)
}

// loadCosignVerifier loads a PEM public key file (cosign.pub format: PKIX
// "PUBLIC KEY") into a signature verifier.
func loadCosignVerifier(pubKeyPath string) (signature.Verifier, error) {
	raw, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return nil, fmt.Errorf("signing: cosign: read key %s: %w", pubKeyPath, err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("signing: cosign: invalid PEM in %s", pubKeyPath)
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("signing: cosign: parse public key: %w", err)
	}
	verifier, err := signature.LoadVerifier(pub, crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("signing: cosign: unsupported key type %T: %w", pub, err)
	}
	return verifier, nil
}

// verifyCosignBundle verifies one Sigstore bundle layer against the expected
// image digest through sigstore-go, trusting only the provided static key.
// WithNoObserverTimestamps is the sigstore-go mode for key (non-certificate)
// verification: a bare cosign key signature carries no transparency log,
// Fulcio certificate, or timestamp authority data, and requiring any of
// those would reject every valid key-signed image.
func verifyCosignBundle(bundleJSON []byte, digestHex string, verifier signature.Verifier) error {
	pb := &v1.Bundle{}
	if err := protojson.Unmarshal(bundleJSON, pb); err != nil {
		return fmt.Errorf("parse bundle: %w", err)
	}
	entity, err := bundle.NewBundle(pb)
	if err != nil {
		return fmt.Errorf("load bundle: %w", err)
	}
	digestBytes, err := hex.DecodeString(digestHex)
	if err != nil {
		return fmt.Errorf("decode digest: %w", err)
	}
	// The bundle names its key by hint; thrive trusts exactly one static
	// key, so resolve any hint to it. Soundness comes from the cryptographic
	// signature check, not the hint string.
	key := root.NewExpiringKey(verifier, time.Unix(0, 0).UTC(), time.Now().AddDate(100, 0, 0).UTC())
	trusted := root.NewTrustedPublicKeyMaterial(func(string) (root.TimeConstrainedVerifier, error) {
		return key, nil
	})
	sv, err := verify.NewSignedEntityVerifier(trusted, verify.WithNoObserverTimestamps())
	if err != nil {
		return fmt.Errorf("verifier: %w", err)
	}
	policy := verify.NewPolicy(verify.WithArtifactDigest("sha256", digestBytes), verify.WithKey())
	if _, err := sv.Verify(entity, policy); err != nil {
		return err
	}
	return nil
}

// looksLikeSimpleSigning reports whether data is a cosign SimpleSigning
// payload (legacy signature format).
func looksLikeSimpleSigning(data []byte) bool {
	var v struct {
		Critical map[string]any `json:"critical"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return false
	}
	return v.Critical != nil
}

// verifySimpleSigning verifies a legacy cosign SimpleSigning payload against
// its base64 signature annotation with the static key.
func verifySimpleSigning(payload []byte, sigB64 string, verifier signature.Verifier) error {
	if sigB64 == "" {
		return fmt.Errorf("missing %s annotation", cosignSignatureAnnotation)
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	if err := verifier.VerifySignature(bytes.NewReader(sig), bytes.NewReader(payload)); err != nil {
		return fmt.Errorf("signature invalid: %w", err)
	}
	return nil
}
