// Package signing provides Ed25519-based OCI image signing and verification.
package signing

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// KeyPair holds an Ed25519 signing key pair.
type KeyPair struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

// Signature is stored alongside an image manifest.
type Signature struct {
	ImageDigest string    `json:"image_digest"`
	SignedAt    time.Time `json:"signed_at"`
	PublicKey   string    `json:"public_key_hex"`
	Signature   string    `json:"signature_hex"`
}

// GenerateKeyPair creates a new Ed25519 key pair and saves it to ~/.thrive/keys/.
func GenerateKeyPair(keyName string) (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("signing: generate key: %w", err)
	}
	dir := keyDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("signing: mkdir keys: %w", err)
	}
	privBlock := &pem.Block{Type: "ED25519 PRIVATE KEY", Bytes: priv}
	pubBlock := &pem.Block{Type: "ED25519 PUBLIC KEY", Bytes: pub}
	if err := os.WriteFile(filepath.Join(dir, keyName+".priv"), pem.EncodeToMemory(privBlock), 0600); err != nil {
		return nil, fmt.Errorf("signing: write private key: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, keyName+".pub"), pem.EncodeToMemory(pubBlock), 0644); err != nil {
		return nil, fmt.Errorf("signing: write public key: %w", err)
	}
	return &KeyPair{PrivateKey: priv, PublicKey: pub}, nil
}

// LoadKeyPair loads an Ed25519 key pair from ~/.thrive/keys/<keyName>.{priv,pub}.
func LoadKeyPair(keyName string) (*KeyPair, error) {
	dir := keyDir()
	privData, err := os.ReadFile(filepath.Join(dir, keyName+".priv"))
	if err != nil {
		return nil, fmt.Errorf("signing: read private key %s: %w", keyName, err)
	}
	pubData, err := os.ReadFile(filepath.Join(dir, keyName+".pub"))
	if err != nil {
		return nil, fmt.Errorf("signing: read public key %s: %w", keyName, err)
	}
	privBlock, _ := pem.Decode(privData)
	pubBlock, _ := pem.Decode(pubData)
	if privBlock == nil || pubBlock == nil {
		return nil, fmt.Errorf("signing: invalid PEM for key %s", keyName)
	}
	return &KeyPair{
		PrivateKey: ed25519.PrivateKey(privBlock.Bytes),
		PublicKey:  ed25519.PublicKey(pubBlock.Bytes),
	}, nil
}

// Sign signs imageDigest with kp and writes the signature JSON to sigPath.
func Sign(imageDigest, sigPath string, kp *KeyPair) (*Signature, error) {
	msg := digestBytes(imageDigest)
	sigBytes := ed25519.Sign(kp.PrivateKey, msg)
	sig := &Signature{
		ImageDigest: imageDigest,
		SignedAt:    time.Now().UTC(),
		PublicKey:   hex.EncodeToString(kp.PublicKey),
		Signature:   hex.EncodeToString(sigBytes),
	}
	data, err := json.MarshalIndent(sig, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("signing: marshal: %w", err)
	}
	if err := os.WriteFile(sigPath, data, 0644); err != nil {
		return nil, fmt.Errorf("signing: write sig: %w", err)
	}
	return sig, nil
}

// Verify checks the signature at sigPath against imageDigest.
func Verify(imageDigest, sigPath string) error {
	data, err := os.ReadFile(sigPath)
	if err != nil {
		return fmt.Errorf("signing: read sig: %w", err)
	}
	var sig Signature
	if err := json.Unmarshal(data, &sig); err != nil {
		return fmt.Errorf("signing: parse sig: %w", err)
	}
	if sig.ImageDigest != imageDigest {
		return fmt.Errorf("signing: digest mismatch: sig=%s want=%s", sig.ImageDigest, imageDigest)
	}
	pubKeyBytes, err := hex.DecodeString(sig.PublicKey)
	if err != nil {
		return fmt.Errorf("signing: decode public key: %w", err)
	}
	sigBytes, err := hex.DecodeString(sig.Signature)
	if err != nil {
		return fmt.Errorf("signing: decode signature: %w", err)
	}
	if !ed25519.Verify(ed25519.PublicKey(pubKeyBytes), digestBytes(imageDigest), sigBytes) {
		return fmt.Errorf("signing: verification FAILED for %s", imageDigest)
	}
	return nil
}

// SigPath returns where a signature for imageDigest is stored inside imgDir.
func SigPath(imgDir, imageDigest string) string {
	safe := hex.EncodeToString([]byte(imageDigest))
	if len(safe) > 16 {
		safe = safe[:16]
	}
	return filepath.Join(imgDir, safe+".sig")
}

func keyDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".thrive", "keys")
}

// digestBytes converts an image digest string to the byte slice that is signed.
func digestBytes(digest string) []byte {
	raw := digest
	if len(raw) > 7 && raw[:7] == "sha256:" {
		raw = raw[7:]
	}
	b, err := hex.DecodeString(raw)
	if err != nil {
		h := sha256.Sum256([]byte(digest))
		return h[:]
	}
	return b
}
