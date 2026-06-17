//go:build linux
// +build linux

package secrets

import (
	"encoding/hex"
	"sync"
	"testing"
)

// TestEncrypt_WrongKey verifies that ciphertext produced with key A cannot
// be decrypted with a different key B.  AES-GCM authentication detects the
// mismatch and must return an error.
func TestEncrypt_WrongKey(t *testing.T) {
	// Arrange — encrypt with key A.
	keyA := make([]byte, 32)
	for i := range keyA {
		keyA[i] = byte(i + 1)
	}
	t.Setenv("THRIVE_MASTER_KEY", hex.EncodeToString(keyA))
	resetKeyState()

	plaintext := []byte("super secret value")

	ciphertext, err := Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt (keyA): unexpected error: %v", err)
	}

	// Switch to a distinct key B for decryption.
	keyB := make([]byte, 32)
	for i := range keyB {
		keyB[i] = byte(i + 100) // distinct from keyA
	}
	t.Setenv("THRIVE_MASTER_KEY", hex.EncodeToString(keyB))
	resetKeyState()
	t.Cleanup(resetKeyState)

	// Act — attempt to decrypt with key B.
	_, err = Decrypt(ciphertext)

	// Assert — GCM authentication tag must fail.
	if err == nil {
		t.Fatal("Decrypt with wrong key: expected error, got nil")
	}
}

// TestEncrypt_CorruptCiphertext verifies that Decrypt returns an error when
// the ciphertext bytes are modified after encryption.  AES-GCM AEAD
// authentication must detect any bit-level corruption.
func TestEncrypt_CorruptCiphertext(t *testing.T) {
	// Arrange
	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = 0xDE
	}
	t.Setenv("THRIVE_MASTER_KEY", hex.EncodeToString(rawKey))
	resetKeyState()
	t.Cleanup(resetKeyState)

	plaintext := []byte("data that will be corrupted")

	ciphertext, err := Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: unexpected error: %v", err)
	}

	// Act — flip all bits in a byte past the nonce (standard GCM nonce = 12 bytes).
	corrupted := make([]byte, len(ciphertext))
	copy(corrupted, ciphertext)
	const gcmNonceSize = 12
	if len(corrupted) > gcmNonceSize {
		corrupted[gcmNonceSize] ^= 0xFF
	} else {
		corrupted[len(corrupted)-1] ^= 0xFF
	}

	_, err = Decrypt(corrupted)

	// Assert
	if err == nil {
		t.Fatal("Decrypt with corrupted ciphertext: expected error, got nil")
	}
}

// TestEncrypt_TruncatedCiphertext verifies that Decrypt returns an error when
// the input is shorter than the minimum valid length (nonce size).
func TestEncrypt_TruncatedCiphertext(t *testing.T) {
	// Arrange
	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = 0xBE
	}
	t.Setenv("THRIVE_MASTER_KEY", hex.EncodeToString(rawKey))
	resetKeyState()
	t.Cleanup(resetKeyState)

	// Act — pass a 3-byte slice, well below the 12-byte GCM nonce minimum.
	truncated := []byte{0x01, 0x02, 0x03}
	_, err := Decrypt(truncated)

	// Assert
	if err == nil {
		t.Fatal("Decrypt with truncated data: expected error, got nil")
	}
}

// TestVault_AutoKeyGeneration verifies that getMasterKey returns the same
// 32-byte key on two consecutive calls — the sync.Once guarantee means the
// key is generated exactly once per key lifetime.
func TestVault_AutoKeyGeneration(t *testing.T) {
	// Arrange — clear the env var so auto-generation is triggered.
	t.Setenv("THRIVE_MASTER_KEY", "")
	resetKeyState()
	t.Cleanup(resetKeyState)

	// Act — call getMasterKey twice.
	key1, err := getMasterKey()
	if err != nil {
		t.Fatalf("getMasterKey (first call): %v", err)
	}

	key2, err := getMasterKey()
	if err != nil {
		t.Fatalf("getMasterKey (second call): %v", err)
	}

	// Assert — key must be 32 bytes and identical across both calls.
	if len(key1) != 32 {
		t.Errorf("key1 length: got %d, want 32", len(key1))
	}
	if len(key2) != 32 {
		t.Errorf("key2 length: got %d, want 32", len(key2))
	}
	for i := range key1 {
		if key1[i] != key2[i] {
			t.Errorf("keys differ at byte %d: 0x%02x vs 0x%02x", i, key1[i], key2[i])
			break
		}
	}
}

// TestEncrypt_ConcurrentSafety verifies that concurrent Encrypt/Decrypt calls
// with a shared key do not race or corrupt each other's output.
func TestEncrypt_ConcurrentSafety(t *testing.T) {
	// Arrange
	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = byte(i + 50)
	}
	t.Setenv("THRIVE_MASTER_KEY", hex.EncodeToString(rawKey))
	resetKeyState()
	t.Cleanup(resetKeyState)

	plaintext := []byte("concurrent encryption payload")
	const workers = 8

	errs := make([]error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)

	// Act — launch workers concurrently.
	for i := 0; i < workers; i++ {
		idx := i
		go func() {
			defer wg.Done()
			ct, encErr := Encrypt(plaintext)
			if encErr != nil {
				errs[idx] = encErr
				return
			}
			_, errs[idx] = Decrypt(ct)
		}()
	}
	wg.Wait()

	// Assert — every worker must succeed.
	for i, err := range errs {
		if err != nil {
			t.Errorf("worker %d: %v", i, err)
		}
	}
}
