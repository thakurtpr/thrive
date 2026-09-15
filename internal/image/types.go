// Package image provides cross-platform OCI image type definitions.
package image

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// SafeRef converts an image reference to a safe filesystem directory name.
// Used on both macOS and Linux so virtiofs-shared images have matching paths.
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
}

// PushOptions configures image push behavior.
type PushOptions struct {
	Username  string
	Password  string
	PlainHTTP bool
}
