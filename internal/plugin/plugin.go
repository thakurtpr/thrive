//go:build linux
// +build linux

// Package plugin manages engine plugins (docker plugin parity,
// management plane): install from a directory or tarball, enable,
// disable, inspect, list, remove. Execution hooks into the container
// lifecycle are a documented future step; state transitions are real.
package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/thakurprasadrout/thrive/internal/registry"
)

// Plugin is an installed plugin record.
type Plugin struct {
	Name    string    `json:"name"`
	Enabled bool      `json:"enabled"`
	Created time.Time `json:"created"`
}

var pluginsDirOverride string

func pluginsDir() string {
	if pluginsDirOverride != "" {
		return pluginsDirOverride
	}
	return "/var/lib/thrive/plugins"
}

func pluginDir(name string) string { return filepath.Join(pluginsDir(), name) }

func metaPath(name string) string { return filepath.Join(pluginDir(name), "plugin.json") }

func validName(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			c == '_' || c == '.' || c == '-' || c == '/' || c == ':') {
			return false
		}
	}
	return true
}

// Install installs a plugin from a directory or tarball.
func Install(name, src string) (*Plugin, error) {
	if !validName(name) {
		return nil, fmt.Errorf("plugin: invalid name %q", name)
	}
	if _, err := os.Stat(metaPath(name)); err == nil {
		return nil, fmt.Errorf("plugin: %s already exists", name)
	}
	rootfs := filepath.Join(pluginDir(name), "rootfs")
	if err := os.MkdirAll(rootfs, 0755); err != nil {
		return nil, fmt.Errorf("plugin: mkdir: %w", err)
	}
	fi, err := os.Stat(src)
	if err != nil {
		os.RemoveAll(pluginDir(name)) //nolint:errcheck
		return nil, fmt.Errorf("plugin: stat %s: %w", src, err)
	}
	if fi.IsDir() {
		if err := copyTree(src, rootfs); err != nil {
			os.RemoveAll(pluginDir(name)) //nolint:errcheck
			return nil, fmt.Errorf("plugin: copy: %w", err)
		}
	} else {
		f, err := os.Open(src)
		if err != nil {
			os.RemoveAll(pluginDir(name)) //nolint:errcheck
			return nil, fmt.Errorf("plugin: open: %w", err)
		}
		err = registry.ExtractArchive(f, rootfs)
		f.Close()
		if err != nil {
			os.RemoveAll(pluginDir(name)) //nolint:errcheck
			return nil, fmt.Errorf("plugin: extract: %w", err)
		}
	}
	p := &Plugin{Name: name, Created: time.Now().UTC()}
	if err := writePlugin(p); err != nil {
		os.RemoveAll(pluginDir(name)) //nolint:errcheck
		return nil, err
	}
	return p, nil
}

// Enable marks a plugin enabled.
func Enable(name string) error {
	p, err := Inspect(name)
	if err != nil {
		return err
	}
	p.Enabled = true
	return writePlugin(p)
}

// Disable marks a plugin disabled.
func Disable(name string) error {
	p, err := Inspect(name)
	if err != nil {
		return err
	}
	p.Enabled = false
	return writePlugin(p)
}

// Inspect returns a plugin by name.
func Inspect(name string) (*Plugin, error) {
	data, err := os.ReadFile(metaPath(name))
	if err != nil {
		return nil, fmt.Errorf("plugin: %s not found: %w", name, err)
	}
	var p Plugin
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("plugin: parse: %w", err)
	}
	return &p, nil
}

// List returns all plugins.
func List() ([]*Plugin, error) {
	entries, err := os.ReadDir(pluginsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("plugin: list: %w", err)
	}
	var out []*Plugin
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if p, err := Inspect(e.Name()); err == nil {
			out = append(out, p)
		}
	}
	return out, nil
}

// Remove deletes a plugin. Enabled plugins need --force.
func Remove(name string, force bool) error {
	p, err := Inspect(name)
	if err != nil {
		return err
	}
	if p.Enabled && !force {
		return fmt.Errorf("plugin: %s is enabled (disable first or use --force)", name)
	}
	if err := os.RemoveAll(pluginDir(name)); err != nil {
		return fmt.Errorf("plugin: remove: %w", err)
	}
	return nil
}

func writePlugin(p *Plugin) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("plugin: marshal: %w", err)
	}
	if err := os.WriteFile(metaPath(p.Name), data, 0644); err != nil {
		return fmt.Errorf("plugin: write: %w", err)
	}
	return nil
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return os.MkdirAll(dst, 0755)
		}
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, fi.Mode())
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fi.Mode())
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}
