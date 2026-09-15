// Package buildx manages extended build capabilities: named builders,
// compose-file bake targets, and build-cache accounting. Portable: builder
// records live in the home directory on every platform; cache accounting
// takes an explicit directory so the daemon supplies the VM path.
package buildx

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Builder is a named build driver record.
type Builder struct {
	Name     string    `json:"name"`
	Driver   string    `json:"driver"`
	Endpoint string    `json:"endpoint,omitempty"`
	Current  bool      `json:"-"`
	Created  time.Time `json:"created"`
}

var buildersDirOverride string
var currentFileOverride string

func buildersDir() string {
	if buildersDirOverride != "" {
		return buildersDirOverride
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".thrive", "builders")
}

func currentFile() string {
	if currentFileOverride != "" {
		return currentFileOverride
	}
	return filepath.Join(buildersDir(), "current")
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

// CreateBuilder records a builder (driver docker-container or thrive).
func CreateBuilder(name, driver, endpoint string) (*Builder, error) {
	if !validName(name) {
		return nil, fmt.Errorf("buildx: invalid builder name %q", name)
	}
	if driver == "" {
		driver = "thrive"
	}
	if driver != "thrive" && driver != "docker-container" {
		return nil, fmt.Errorf("buildx: driver %q not supported (thrive, docker-container)", driver)
	}
	path := filepath.Join(buildersDir(), name+".json")
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("buildx: builder %s already exists", name)
	}
	b := &Builder{Name: name, Driver: driver, Endpoint: endpoint, Created: time.Now().UTC()}
	if err := writeBuilder(b); err != nil {
		return nil, err
	}
	// First builder becomes current.
	if _, err := os.Stat(currentFile()); os.IsNotExist(err) {
		UseBuilder(name) //nolint:errcheck
	}
	return b, nil
}

// InspectBuilder returns a builder by name.
func InspectBuilder(name string) (*Builder, error) {
	data, err := os.ReadFile(filepath.Join(buildersDir(), name+".json"))
	if err != nil {
		return nil, fmt.Errorf("buildx: builder %s not found: %w", name, err)
	}
	var b Builder
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("buildx: parse builder: %w", err)
	}
	if cur, err := os.ReadFile(currentFile()); err == nil && string(cur) == name {
		b.Current = true
	}
	return &b, nil
}

// ListBuilders returns all builders.
func ListBuilders() ([]*Builder, error) {
	entries, err := os.ReadDir(buildersDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("buildx: list: %w", err)
	}
	var out []*Builder
	for _, e := range entries {
		if e.IsDir() || len(e.Name()) < 6 || e.Name()[len(e.Name())-5:] != ".json" {
			continue
		}
		if b, err := InspectBuilder(e.Name()[:len(e.Name())-5]); err == nil {
			out = append(out, b)
		}
	}
	return out, nil
}

// RemoveBuilder deletes a builder record.
func RemoveBuilder(name string) error {
	if err := os.Remove(filepath.Join(buildersDir(), name+".json")); err != nil {
		return fmt.Errorf("buildx: builder %s not found: %w", name, err)
	}
	if cur, err := os.ReadFile(currentFile()); err == nil && string(cur) == name {
		os.Remove(currentFile()) //nolint:errcheck
	}
	return nil
}

// UseBuilder sets the current builder.
func UseBuilder(name string) error {
	if _, err := InspectBuilder(name); err != nil {
		return err
	}
	if err := os.MkdirAll(buildersDir(), 0755); err != nil {
		return fmt.Errorf("buildx: mkdir: %w", err)
	}
	if err := os.WriteFile(currentFile(), []byte(name), 0644); err != nil {
		return fmt.Errorf("buildx: write current: %w", err)
	}
	return nil
}

// CurrentBuilder returns the current builder name, or "".
func CurrentBuilder() string {
	data, err := os.ReadFile(currentFile())
	if err != nil {
		return ""
	}
	return string(data)
}

// CacheUsage returns total bytes under a build cache directory.
func CacheUsage(cacheDir string) int64 {
	var total int64
	filepath.Walk(cacheDir, func(_ string, fi os.FileInfo, err error) error { //nolint:errcheck
		if err == nil && fi.Mode().IsRegular() {
			total += fi.Size()
		}
		return nil
	})
	return total
}

// PruneCache removes all cache entries, returning reclaimed bytes.
func PruneCache(cacheDir string) (int64, error) {
	reclaimed := CacheUsage(cacheDir)
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("buildx: prune: %w", err)
	}
	for _, e := range entries {
		os.RemoveAll(filepath.Join(cacheDir, e.Name())) //nolint:errcheck
	}
	return reclaimed, nil
}

func writeBuilder(b *Builder) error {
	if err := os.MkdirAll(buildersDir(), 0755); err != nil {
		return fmt.Errorf("buildx: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("buildx: marshal: %w", err)
	}
	if err := os.WriteFile(filepath.Join(buildersDir(), b.Name+".json"), data, 0644); err != nil {
		return fmt.Errorf("buildx: write: %w", err)
	}
	return nil
}
