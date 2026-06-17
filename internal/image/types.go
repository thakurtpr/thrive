// Package image provides cross-platform OCI image type definitions.
package image

import "strings"

// SafeRef converts an image reference to a safe filesystem directory name.
// Used on both macOS and Linux so virtiofs-shared images have matching paths.
func SafeRef(ref string) string {
	return strings.NewReplacer("/", "_", ":", "_", "@", "_").Replace(ref)
}

// Image represents a pulled OCI image.
type Image struct {
	Ref    string
	Digest string
	Layers []Layer
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
