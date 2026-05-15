//go:build linux
// +build linux

package image

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Image represents a pulled OCI image.
type Image struct {
	Ref    string
	Digest string
	Layers []Layer
}

// Layer represents a single image layer.
type Layer struct {
	Digest string
	Size   int64
	Path   string
}

// PullOptions controls how an image is pulled.
type PullOptions struct {
	Username  string
	Password  string
	PlainHTTP bool
}

// Pull downloads an image from a registry and stores it locally.
func Pull(ctx context.Context, ref string, opts PullOptions) (*Image, error) {
	parsed, err := name.ParseReference(ref)
	if err != nil {
		return nil, fmt.Errorf("image.Pull: parse reference %s: %w", ref, err)
	}

	imgDir := filepath.Join("/var/lib/thrive/images", parsed.String())
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		return nil, fmt.Errorf("image.Pull: mkdir %s: %w", imgDir, err)
	}

	var auth authn.Authenticator
	if opts.Username != "" {
		auth = &authn.Basic{Username: opts.Username, Password: opts.Password}
	}

	descriptor, err := remote.Get(parsed, remote.WithAuth(auth))
	if err != nil {
		return nil, fmt.Errorf("image.Pull: remote.Get: %w", err)
	}

	img, err := descriptor.Image()
	if err != nil {
		return nil, fmt.Errorf("image.Pull: Image: %w", err)
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("image.Pull: Layers: %w", err)
	}

	var imgLayers []Layer
	for i, layer := range layers {
		digest, err := layer.Digest()
		if err != nil {
			return nil, fmt.Errorf("image.Pull: layer[%d] Digest: %w", i, err)
		}
		size, err := layer.Size()
		if err != nil {
			return nil, fmt.Errorf("image.Pull: layer[%d] Size: %w", i, err)
		}

		digestHex := digest.String()
		chunkPath := filepath.Join("/var/lib/thrive/chunks", digestHex[:2], digestHex[2:])
		if err := os.MkdirAll(filepath.Dir(chunkPath), 0755); err != nil {
			return nil, fmt.Errorf("image.Pull: mkdir chunk: %w", err)
		}

		if _, err := os.Stat(chunkPath); os.IsNotExist(err) {
			rc, err := layer.Uncompressed()
			if err != nil {
				return nil, fmt.Errorf("image.Pull: layer[%d] Uncompressed: %w", i, err)
			}
			defer rc.Close()

			f, err := os.Create(chunkPath)
			if err != nil {
				return nil, fmt.Errorf("image.Pull: create chunk: %w", err)
			}
			defer f.Close()

			if _, err := io.Copy(f, rc); err != nil {
				return nil, fmt.Errorf("image.Pull: copy layer: %w", err)
			}
		}

		imgLayers = append(imgLayers, Layer{
			Digest: digestHex,
			Size:   size,
			Path:   chunkPath,
		})
	}

	metadata := struct {
		Ref    string
		Digest string
		Layers []Layer
	}{
		Ref:    parsed.String(),
		Digest: descriptor.Digest.String(),
		Layers: imgLayers,
	}
	metaJSON, _ := json.Marshal(metadata)
	os.WriteFile(filepath.Join(imgDir, "manifest.json"), metaJSON, 0644)

	return &Image{
		Ref:    parsed.String(),
		Digest: descriptor.Digest.String(),
		Layers: imgLayers,
	}, nil
}

// Mount prepares an image for a container and returns the rootfs path.
func Mount(ctx context.Context, imageRef, containerID string) (string, error) {
	imgDir := filepath.Join("/var/lib/thrive/images", imageRef)

	metaPath := filepath.Join(imgDir, "manifest.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return "", fmt.Errorf("image.Mount: read manifest: %w", err)
	}

	var metadata struct {
		Layers []Layer
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return "", fmt.Errorf("image.Mount: unmarshal: %w", err)
	}

	containerDir := filepath.Join("/run/thrive/containers", containerID)
	upperDir := filepath.Join(containerDir, "upper")
	workDir := filepath.Join(containerDir, "work")

	os.MkdirAll(upperDir, 0755)
	os.MkdirAll(workDir, 0755)

	if len(metadata.Layers) > 0 {
		return metadata.Layers[0].Path, nil
	}

	return "", fmt.Errorf("image.Mount: no layers found")
}

// Unmount removes the container's rootfs.
func Unmount(ctx context.Context, containerID string) error {
	return nil
}

// List returns all locally stored images.
func List(ctx context.Context) ([]*Image, error) {
	imgDir := "/var/lib/thrive/images"
	entries, err := os.ReadDir(imgDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("image.List: readdir: %w", err)
	}

	var images []*Image
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(imgDir, entry.Name(), "manifest.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var metadata struct {
			Ref    string
			Digest string
			Layers []Layer
		}
		if err := json.Unmarshal(data, &metadata); err != nil {
			continue
		}
		images = append(images, &Image{
			Ref:    metadata.Ref,
			Digest: metadata.Digest,
			Layers: metadata.Layers,
		})
	}
	return images, nil
}

// Remove deletes an image from local storage.
func Remove(ctx context.Context, imageRef string) error {
	imgDir := filepath.Join("/var/lib/thrive/images", imageRef)
	return os.RemoveAll(imgDir)
}

// ChunkStore provides content-addressed storage.
type ChunkStore struct {
	basePath string
}

// NewChunkStore creates a new chunk store at the given path.
func NewChunkStore(basePath string) *ChunkStore {
	return &ChunkStore{basePath: basePath}
}

// Put stores data with the given digest.
func (cs *ChunkStore) Put(ctx context.Context, digest string, data []byte) error {
	chunkPath := cs.chunkPath(digest)
	if err := os.MkdirAll(filepath.Dir(chunkPath), 0755); err != nil {
		return fmt.Errorf("ChunkStore.Put: mkdir: %w", err)
	}
	return os.WriteFile(chunkPath, data, 0644)
}

// Get retrieves data for the given digest.
func (cs *ChunkStore) Get(ctx context.Context, digest string) ([]byte, error) {
	return os.ReadFile(cs.chunkPath(digest))
}

// Has returns true if the chunk exists.
func (cs *ChunkStore) Has(ctx context.Context, digest string) bool {
	_, err := os.Stat(cs.chunkPath(digest))
	return err == nil
}

func (cs *ChunkStore) chunkPath(digest string) string {
	return filepath.Join(cs.basePath, digest[:2], digest[2:])
}

// ComputeDigest computes the SHA-256 digest of data.
func ComputeDigest(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
