//go:build linux
// +build linux

package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/thakurprasadrout/thrive/internal/telemetry"
)

var (
	masterKey []byte
	keyOnce   sync.Once
	keyErr    error
)

const keyLen = 32 // AES-256

const masterKeyPath = "/var/lib/thrive/secrets/.master"

func getMasterKey() ([]byte, error) {
	keyOnce.Do(func() {
		// 1. Prefer explicit env var (CI / operator-provided).
		hexKey := os.Getenv("THRIVE_MASTER_KEY")

		// 2. Fall back to persisted key on disk.
		if hexKey == "" {
			if data, err := os.ReadFile(masterKeyPath); err == nil {
				hexKey = strings.TrimSpace(string(data))
				telemetry.Debug("vault.getMasterKey: loaded persisted key")
			}
		}

		// 3. Generate, persist, and use a fresh key on first run.
		if hexKey == "" {
			raw := make([]byte, keyLen)
			if _, err := io.ReadFull(rand.Reader, raw); err != nil {
				keyErr = fmt.Errorf("vault.getMasterKey: generate key: %w", err)
				return
			}
			hexKey = hex.EncodeToString(raw)
			if err := os.MkdirAll("/var/lib/thrive/secrets", 0700); err == nil {
				os.WriteFile(masterKeyPath, []byte(hexKey), 0600)
			}
			telemetry.Debug("vault.getMasterKey: generated new master key")
		}

		masterKey, keyErr = hex.DecodeString(hexKey)
		if keyErr != nil {
			telemetry.Error("vault.getMasterKey: hex decode failed", telemetry.FieldError(keyErr))
			return
		}
		if len(masterKey) != keyLen {
			keyErr = fmt.Errorf("master key must be %d bytes, got %d", keyLen, len(masterKey))
			return
		}
		telemetry.Debug("vault.getMasterKey: key ready", telemetry.FieldInt("len", len(masterKey)))
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