// Package image provides cross-platform OCI image type definitions.
package image

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// StoreDir returns the platform image-store directory:
// /var/lib/thrive/images on Linux, ~/.thrive/images elsewhere.
func StoreDir() string {
	if runtime.GOOS == "linux" {
		return "/var/lib/thrive/images"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".thrive", "images")
}

// ParsePlatform parses "os/arch" (docker --platform parity).
func ParsePlatform(s string) (v1.Platform, error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return v1.Platform{}, fmt.Errorf("invalid platform %q (want os/arch)", s)
	}
	return v1.Platform{OS: parts[0], Architecture: parts[1]}, nil
}
// Used on both macOS and Linux so virtiofs-shared images have matching paths.
// SafeRef converts an image reference to a safe filesystem directory name.
func SafeRef(ref string) string {
	return strings.NewReplacer("/", "_", ":", "_", "@", "_").Replace(ref)
}

// Image represents a pulled OCI image.
type Image struct {
	Ref        string
	Digest     string
	Layers     []Layer
	Env        []string // image-default environment variables (from OCI config)
	Entrypoint []string // image-default entrypoint (from OCI config)
	Cmd        []string // image-default cmd (from OCI config)
}

// Layer represents a single OCI image layer.
type Layer struct {
	Digest string
	Size   int64
	Path   string
}

// PullOptions configures image pull behavior.
type PullOptions struct {
	Username  string
	Password  string
	PlainHTTP bool
	Platform  string // "os/arch"; empty selects the execution default
}

// PushOptions configures image push behavior.
type PushOptions struct {
	Username  string
	Password  string
	PlainHTTP bool
}
