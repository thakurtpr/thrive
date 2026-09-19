//go:build linux
// +build linux

// Package swarmconfig manages swarm config objects (docker config parity):
// named plaintext blobs stored on disk and mounted into containers via
// --config name:target (tmpfs + bind-mount, same pattern as secrets).
package swarmconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config is a stored config object.
type Config struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	Created time.Time `json:"created"`
}

var storeDirOverride string

func storeDir() string {
	if storeDirOverride != "" {
		return storeDirOverride
	}
	return "/var/lib/thrive/configs"
}

func validName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, c := range name {
		if !isNameChar(c) {
			return false
		}
	}
	return true
}

func isNameChar(c rune) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	switch c {
	case '_', '.', '-':
		return true
	}
	return false
}

func dataPath(name string) string { return filepath.Join(storeDir(), name+".data") }
func metaPath(name string) string { return filepath.Join(storeDir(), name+".json") }

// Create stores data under name from a file, directory is rejected.
func Create(name string, data []byte) (*Config, error) {
	if !validName(name) {
		return nil, fmt.Errorf("config: invalid name %q", name)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("config: empty data")
	}
	if _, err := os.Stat(metaPath(name)); err == nil {
		return nil, fmt.Errorf("config: %s already exists", name)
	}
	if err := os.MkdirAll(storeDir(), 0755); err != nil {
		return nil, fmt.Errorf("config: mkdir: %w", err)
	}
	if err := os.WriteFile(dataPath(name), data, 0644); err != nil {
		return nil, fmt.Errorf("config: write: %w", err)
	}
	cfg := &Config{Name: name, Size: int64(len(data)), Created: time.Now().UTC()}
	meta, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		os.Remove(dataPath(name)) //nolint:errcheck
		return nil, fmt.Errorf("config: marshal: %w", err)
	}
	if err := os.WriteFile(metaPath(name), meta, 0644); err != nil {
		os.Remove(dataPath(name)) //nolint:errcheck
		return nil, fmt.Errorf("config: write: %w", err)
	}
	return cfg, nil
}

// Get returns the raw data of a config object.
func Get(name string) ([]byte, error) {
	data, err := os.ReadFile(dataPath(name))
	if err != nil {
		return nil, fmt.Errorf("config: %s not found: %w", name, err)
	}
	return data, nil
}

// Inspect returns config metadata.
func Inspect(name string) (*Config, error) {
	data, err := os.ReadFile(metaPath(name))
	if err != nil {
		return nil, fmt.Errorf("config: %s not found: %w", name, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse: %w", err)
	}
	return &cfg, nil
}

// List returns all config objects.
func List() ([]*Config, error) {
	entries, err := os.ReadDir(storeDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("config: list: %w", err)
	}
	var out []*Config
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if cfg, err := Inspect(strings.TrimSuffix(e.Name(), ".json")); err == nil {
			out = append(out, cfg)
		}
	}
	return out, nil
}

// Remove deletes a config object.
func Remove(name string) error {
	if _, err := Inspect(name); err != nil {
		return err
	}
	if err := os.Remove(dataPath(name)); err != nil {
		return fmt.Errorf("config: remove: %w", err)
	}
	if err := os.Remove(metaPath(name)); err != nil {
		return fmt.Errorf("config: remove: %w", err)
	}
	return nil
}
