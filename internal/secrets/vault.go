//go:build linux
// +build linux

package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync"

	"github.com/thakurprasadrout/thrive/internal/telemetry"
)

var (
	masterKey []byte
	keyOnce   sync.Once
	keyErr    error
)

const keyLen = 32 // AES-256

func getMasterKey() ([]byte, error) {
	keyOnce.Do(func() {
		hexKey := os.Getenv("THRIVE_MASTER_KEY")
		if hexKey == "" {
			keyErr = fmt.Errorf("THRIVE_MASTER_KEY env var not set")
			telemetry.Error("vault.getMasterKey: master key missing", telemetry.FieldError(keyErr))
			return
		}
		masterKey, keyErr = hex.DecodeString(hexKey)
		if keyErr != nil {
			telemetry.Error("vault.getMasterKey: hex decode failed", telemetry.FieldError(keyErr))
			return
		}
		if len(masterKey) != keyLen {
			keyErr = fmt.Errorf("master key must be %d bytes, got %d", keyLen, len(masterKey))
			telemetry.Error("vault.getMasterKey: invalid key length", telemetry.FieldInt("expected", keyLen), telemetry.FieldInt("got", len(masterKey)))
			return
		}
		telemetry.Debug("vault.getMasterKey: master key loaded", telemetry.FieldInt("keyLen", len(masterKey)))
	})
	return masterKey, keyErr
}

func Encrypt(plaintext []byte) ([]byte, error) {
	log := telemetry.Logger()
	log.Debug("vault.Encrypt: starting", telemetry.FieldInt("plaintextLen", len(plaintext)))

	key, err := getMasterKey()
	if err != nil {
		return nil, fmt.Errorf("vault.Encrypt: getMasterKey: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		log.Error("vault.Encrypt: aes.NewCipher failed", telemetry.FieldError(err))
		return nil, fmt.Errorf("vault.Encrypt: cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		log.Error("vault.Encrypt: NewGCM failed", telemetry.FieldError(err))
		return nil, fmt.Errorf("vault.Encrypt: gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		log.Error("vault.Encrypt: rand.Read nonce failed", telemetry.FieldError(err))
		return nil, fmt.Errorf("vault.Encrypt: nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	log.Debug("vault.Encrypt: done", telemetry.FieldInt("outputLen", len(ciphertext)))
	return ciphertext, nil
}

func Decrypt(data []byte) ([]byte, error) {
	log := telemetry.Logger()
	log.Debug("vault.Decrypt: starting", telemetry.FieldInt("dataLen", len(data)))

	key, err := getMasterKey()
	if err != nil {
		return nil, fmt.Errorf("vault.Decrypt: getMasterKey: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		log.Error("vault.Decrypt: aes.NewCipher failed", telemetry.FieldError(err))
		return nil, fmt.Errorf("vault.Decrypt: cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		log.Error("vault.Decrypt: NewGCM failed", telemetry.FieldError(err))
		return nil, fmt.Errorf("vault.Decrypt: gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		log.Error("vault.Decrypt: data too short", telemetry.FieldInt("dataLen", len(data)), telemetry.FieldInt("nonceSize", nonceSize))
		return nil, fmt.Errorf("vault.Decrypt: data too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		log.Error("vault.Decrypt: gcm.Open failed", telemetry.FieldError(err))
		return nil, fmt.Errorf("vault.Decrypt: decryption failed")
	}

	log.Debug("vault.Decrypt: done", telemetry.FieldInt("plaintextLen", len(plaintext)))
	return plaintext, nil
}