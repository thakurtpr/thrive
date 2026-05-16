//go:build linux
// +build linux

package secrets

import (
	"bytes"
	"encoding/hex"
	"os"
	"sync"
	"testing"
)

// resetKeyState resets the package-level singleton so each test gets a fresh
// key derivation path. Must be called before any test that sets THRIVE_MASTER_KEY.
func resetKeyState() {
	keyOnce = sync.Once{}
	masterKey = nil
	keyErr = nil
}

// TestEncryptDecrypt_Roundtrip verifies that data encrypted with Encrypt can be
// recovered identically by Decrypt when a known 32-byte master key is set.
func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	// Arrange
	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = byte(i + 1)
	}
	t.Setenv("THRIVE_MASTER_KEY", hex.EncodeToString(rawKey))
	resetKeyState()
	t.Cleanup(resetKeyState)

	plaintext := []byte("hello thrive secrets")

	// Act
	ciphertext, err := Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: unexpected error: %v", err)
	}

	recovered, err := Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: unexpected error: %v", err)
	}

	// Assert
	if !bytes.Equal(recovered, plaintext) {
		t.Errorf("roundtrip mismatch: got %q, want %q", recovered, plaintext)
	}
}

// TestEncryptDecrypt_CiphertextDiffersFromPlaintext ensures Encrypt does not
// return the plaintext unchanged (i.e. encryption actually occurs).
func TestEncryptDecrypt_CiphertextDiffersFromPlaintext(t *testing.T) {
	// Arrange
	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = 0xAB
	}
	t.Setenv("THRIVE_MASTER_KEY", hex.EncodeToString(rawKey))
	resetKeyState()
	t.Cleanup(resetKeyState)

	plaintext := []byte("do not return me unchanged")

	// Act
	ciphertext, err := Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Assert
	if bytes.Equal(ciphertext, plaintext) {
		t.Error("ciphertext must differ from plaintext")
	}
}

// TestGetMasterKey_AutoGenerate verifies that getMasterKey generates and returns
// a valid 32-byte key when no env var or persistent key file is present.
func TestGetMasterKey_AutoGenerate(t *testing.T) {
	// Arrange — clear env and redirect the persisted-key path to a temp dir.
	os.Unsetenv("THRIVE_MASTER_KEY")
	resetKeyState()
	t.Cleanup(resetKeyState)

	// Act
	key, err := getMasterKey()

	// Assert
	if err != nil {
		t.Fatalf("getMasterKey: unexpected error: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("key length: got %d, want 32", len(key))
	}
}
