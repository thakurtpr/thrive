//go:build linux
// +build linux

package secrets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"github.com/thakurprasadrout/thrive/internal/telemetry"
)

const (
	secretsDir   = "/var/lib/thrive/secrets"
	manifestPath = secretsDir + "/manifest.json"
)

type ManifestEntry struct {
	UUID string `json:"uuid"`
}

type Manifest struct {
	mu      sync.RWMutex
	entries map[string]ManifestEntry
}

var manifest = &Manifest{entries: make(map[string]ManifestEntry)}

func initDirs() error {
	if err := os.MkdirAll(secretsDir, 0700); err != nil {
		return fmt.Errorf("initDirs: mkdir %s: %w", secretsDir, err)
	}
	return nil
}

func loadManifest() error {
	manifest.mu.Lock()
	defer manifest.mu.Unlock()

	data, err := os.ReadFile(manifestPath)
	if os.IsNotExist(err) {
		manifest.entries = make(map[string]ManifestEntry)
		return nil
	}
	if err != nil {
		return fmt.Errorf("loadManifest: read: %w", err)
	}

	if err := json.Unmarshal(data, &manifest.entries); err != nil {
		return fmt.Errorf("loadManifest: unmarshal: %w", err)
	}

	return nil
}

func saveManifest() error {
	manifest.mu.RLock()
	defer manifest.mu.RUnlock()

	data, err := json.MarshalIndent(manifest.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("saveManifest: marshal: %w", err)
	}

	if err := os.WriteFile(manifestPath, data, 0600); err != nil {
		return fmt.Errorf("saveManifest: write: %w", err)
	}

	return nil
}

func Create(name, value string) (string, error) {
	log := telemetry.Logger()
	log.Info("store.Create: starting", telemetry.FieldString("name", name))

	if err := initDirs(); err != nil {
		return "", fmt.Errorf("store.Create: initDirs: %w", err)
	}
	if err := loadManifest(); err != nil {
		return "", fmt.Errorf("store.Create: loadManifest: %w", err)
	}

	manifest.mu.Lock()
	if _, exists := manifest.entries[name]; exists {
		manifest.mu.Unlock()
		return "", fmt.Errorf("store.Create: secret %q already exists", name)
	}

	id := uuid.New().String()
	manifest.entries[name] = ManifestEntry{UUID: id}
	manifest.mu.Unlock()

	encrypted, err := Encrypt([]byte(value))
	if err != nil {
		return "", fmt.Errorf("store.Create: encrypt: %w", err)
	}

	secretPath := filepath.Join(secretsDir, id)
	if err := os.WriteFile(secretPath, encrypted, 0600); err != nil {
		return "", fmt.Errorf("store.Create: write: %w", err)
	}

	if err := saveManifest(); err != nil {
		return "", fmt.Errorf("store.Create: saveManifest: %w", err)
	}

	log.Info("store.Create: completed", telemetry.FieldString("name", name), telemetry.FieldString("uuid", id))
	return id, nil
}

func Get(name string) (string, error) {
	log := telemetry.Logger()
	log.Debug("store.Get: starting", telemetry.FieldString("name", name))

	if err := loadManifest(); err != nil {
		return "", fmt.Errorf("store.Get: loadManifest: %w", err)
	}

	manifest.mu.RLock()
	entry, exists := manifest.entries[name]
	manifest.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("store.Get: secret %q not found", name)
	}

	secretPath := filepath.Join(secretsDir, entry.UUID)
	encrypted, err := os.ReadFile(secretPath)
	if err != nil {
		return "", fmt.Errorf("store.Get: read: %w", err)
	}

	plaintext, err := Decrypt(encrypted)
	if err != nil {
		return "", fmt.Errorf("store.Get: decrypt: %w", err)
	}

	return string(plaintext), nil
}

func List() ([]string, error) {
	if err := loadManifest(); err != nil {
		return nil, fmt.Errorf("store.List: loadManifest: %w", err)
	}

	manifest.mu.RLock()
	names := make([]string, 0, len(manifest.entries))
	for name := range manifest.entries {
		names = append(names, name)
	}
	manifest.mu.RUnlock()

	return names, nil
}

func Delete(name string) error {
	log := telemetry.Logger()
	log.Info("store.Delete: starting", telemetry.FieldString("name", name))

	if err := loadManifest(); err != nil {
		return fmt.Errorf("store.Delete: loadManifest: %w", err)
	}

	manifest.mu.Lock()
	entry, exists := manifest.entries[name]
	if !exists {
		manifest.mu.Unlock()
		return fmt.Errorf("store.Delete: secret %q not found", name)
	}
	delete(manifest.entries, name)
	manifest.mu.Unlock()

	secretPath := filepath.Join(secretsDir, entry.UUID)
	if err := os.Remove(secretPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("store.Delete: remove: %w", err)
	}

	if err := saveManifest(); err != nil {
		return fmt.Errorf("store.Delete: saveManifest: %w", err)
	}

	log.Info("store.Delete: completed", telemetry.FieldString("name", name))
	return nil
}