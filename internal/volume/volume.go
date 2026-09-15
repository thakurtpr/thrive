// Package volume manages named persistent volumes (docker volume parity).
// Portable: volumes are plain directories with JSON metadata. The daemon
// resolves them on Linux; macOS/Windows CLIs proxy to the daemon.
package volume

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thakurprasadrout/thrive/internal/events"
)

// Volume is a named persistent data volume.
type Volume struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Created time.Time `json:"created"`
}

// volumeDirOverride redirects the volume store in tests.
var volumeDirOverride string

// StoreDir returns the volume store root.
func StoreDir() string {
	if volumeDirOverride != "" {
		return volumeDirOverride
	}
	return "/var/lib/thrive/volumes"
}

func validName(name string) bool {
	if name == "" || len(name) > 64 || name == "." || name == ".." {
		return false
	}
	// Single label only: reject slashes (path traversal) outright.
	for _, c := range name {
		if !isNameChar(c) {
			return false
		}
	}
	return true
}

// isNameChar reports whether c is allowed in a resource name.
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

// IsNamedVolume reports whether a -v source is a volume name rather than a
// host path (no slash, not ".", not an existing path).
func IsNamedVolume(source string) bool {
	if source == "" || strings.Contains(source, "/") || source == "." || source == ".." {
		return false
	}
	if _, err := os.Stat(source); err == nil {
		return false
	}
	return validName(source)
}

func metaPath(name string) string {
	return filepath.Join(StoreDir(), name, "volume.json")
}

// Create creates a named volume.
func Create(name string) (*Volume, error) {
	if !validName(name) {
		return nil, fmt.Errorf("volume: invalid name %q", name)
	}
	dir := filepath.Join(StoreDir(), name)
	if _, err := os.Stat(metaPath(name)); err == nil {
		return nil, fmt.Errorf("volume: %s already exists", name)
	}
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0755); err != nil {
		return nil, fmt.Errorf("volume: mkdir: %w", err)
	}
	v := &Volume{Name: name, Path: filepath.Join(dir, "data"), Created: time.Now().UTC()}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("volume: marshal: %w", err)
	}
	if err := os.WriteFile(metaPath(name), data, 0644); err != nil {
		return nil, fmt.Errorf("volume: write: %w", err)
	}
	events.Log("volume", "create", name, nil)
	return v, nil
}

// Ensure returns the volume, creating it if missing (docker -v behaviour).
func Ensure(name string) (*Volume, error) {
	if v, err := Inspect(name); err == nil {
		return v, nil
	}
	return Create(name)
}

// Inspect returns a volume by name.
func Inspect(name string) (*Volume, error) {
	data, err := os.ReadFile(metaPath(name))
	if err != nil {
		return nil, fmt.Errorf("volume: %s not found: %w", name, err)
	}
	var v Volume
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("volume: parse %s: %w", name, err)
	}
	return &v, nil
}

// List returns all volumes.
func List() ([]*Volume, error) {
	entries, err := os.ReadDir(StoreDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("volume: list: %w", err)
	}
	var out []*Volume
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		v, err := Inspect(e.Name())
		if err != nil {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

// Remove deletes a volume. Refuses while in use unless force is set.
func Remove(name string, force bool, inUse func(path string) bool) error {
	v, err := Inspect(name)
	if err != nil {
		return err
	}
	if !force && inUse != nil && inUse(v.Path) {
		return fmt.Errorf("volume: %s is in use (use --force)", name)
	}
	if err := os.RemoveAll(filepath.Join(StoreDir(), name)); err != nil {
		return fmt.Errorf("volume: remove: %w", err)
	}
	events.Log("volume", "destroy", name, nil)
	return nil
}

// Prune removes all unused volumes, returning the removed names.
func Prune(inUse func(path string) bool) ([]string, error) {
	vols, err := List()
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, v := range vols {
		if inUse != nil && inUse(v.Path) {
			continue
		}
		if err := Remove(v.Name, true, nil); err == nil {
			removed = append(removed, v.Name)
		}
	}
	return removed, nil
}
