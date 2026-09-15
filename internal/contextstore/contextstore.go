// Package contextstore manages named daemon contexts (docker context
// parity, management plane). Contexts are local records; the current
// context selects which endpoint future commands address. Portable.
package contextstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Context is a named daemon endpoint record.
type Context struct {
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Endpoint    string    `json:"endpoint"`
	Current     bool      `json:"-"`
	Created     time.Time `json:"created"`
}

var contextsDirOverride string
var currentFileOverride string

func contextsDir() string {
	if contextsDirOverride != "" {
		return contextsDirOverride
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".thrive", "contexts")
}

func currentFile() string {
	if currentFileOverride != "" {
		return currentFileOverride
	}
	return filepath.Join(contextsDir(), "current")
}

// DefaultEndpoint is used when no endpoint is given.
const DefaultEndpoint = "unix:///var/run/thrive-daemon.sock"

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

// Create records a context.
func Create(name, description, endpoint string) (*Context, error) {
	if !validName(name) {
		return nil, fmt.Errorf("context: invalid name %q", name)
	}
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	path := filepath.Join(contextsDir(), name+".json")
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("context: %s already exists", name)
	}
	c := &Context{Name: name, Description: description, Endpoint: endpoint, Created: time.Now().UTC()}
	if err := writeContext(c); err != nil {
		return nil, err
	}
	return c, nil
}

// Inspect returns a context by name.
func Inspect(name string) (*Context, error) {
	data, err := os.ReadFile(filepath.Join(contextsDir(), name+".json"))
	if err != nil {
		return nil, fmt.Errorf("context: %s not found: %w", name, err)
	}
	var c Context
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("context: parse: %w", err)
	}
	if cur, err := os.ReadFile(currentFile()); err == nil && string(cur) == name {
		c.Current = true
	}
	return &c, nil
}

// List returns all contexts (default always present).
func List() ([]*Context, error) {
	def := &Context{Name: "default", Description: "Local daemon", Endpoint: DefaultEndpoint}
	if cur, err := os.ReadFile(currentFile()); err != nil || string(cur) == "default" || string(cur) == "" {
		def.Current = true
	}
	out := []*Context{def}
	entries, err := os.ReadDir(contextsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("context: list: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || e.Name() == "current" || e.Name() == "default.json" {
			continue
		}
		if len(e.Name()) < 6 || e.Name()[len(e.Name())-5:] != ".json" {
			continue
		}
		if c, err := Inspect(e.Name()[:len(e.Name())-5]); err == nil {
			out = append(out, c)
		}
	}
	return out, nil
}

// Remove deletes a context.
func Remove(name string) error {
	if name == "default" {
		return fmt.Errorf("context: cannot remove the default context")
	}
	if err := os.Remove(filepath.Join(contextsDir(), name+".json")); err != nil {
		return fmt.Errorf("context: %s not found: %w", name, err)
	}
	if cur, err := os.ReadFile(currentFile()); err == nil && string(cur) == name {
		os.Remove(currentFile()) //nolint:errcheck
	}
	return nil
}

// Use sets the current context.
func Use(name string) error {
	if name != "default" {
		if _, err := Inspect(name); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(contextsDir(), 0755); err != nil {
		return fmt.Errorf("context: mkdir: %w", err)
	}
	if err := os.WriteFile(currentFile(), []byte(name), 0644); err != nil {
		return fmt.Errorf("context: write current: %w", err)
	}
	return nil
}

// Current returns the current context name ("default" when unset).
func Current() string {
	data, err := os.ReadFile(currentFile())
	if err != nil || len(data) == 0 {
		return "default"
	}
	return string(data)
}

func writeContext(c *Context) error {
	if err := os.MkdirAll(contextsDir(), 0755); err != nil {
		return fmt.Errorf("context: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("context: marshal: %w", err)
	}
	if err := os.WriteFile(filepath.Join(contextsDir(), c.Name+".json"), data, 0644); err != nil {
		return fmt.Errorf("context: write: %w", err)
	}
	return nil
}
