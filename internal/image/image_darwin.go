//go:build darwin

// On macOS, images are pulled and stored in ~/.thrive/images/ on the HOST,
// then shared with the Linux VM via virtiofs at /var/lib/thrive/images/.
package image

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// hostImageStore returns the macOS host path for image storage.
func hostImageStore() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".thrive", "images")
}

// Pull downloads an OCI image and extracts layers to ~/.thrive/images/.
// The Linux VM reads the same layout via a virtiofs mount.
func Pull(ctx context.Context, ref string, opts PullOptions) (*Image, error) {
	parsed, err := name.ParseReference(ref)
	if err != nil {
		return nil, fmt.Errorf("image.Pull: parse %s: %w", ref, err)
	}

	imgDir := filepath.Join(hostImageStore(), SafeRef(parsed.String()))
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		return nil, fmt.Errorf("image.Pull: mkdir: %w", err)
	}

	// Return cached image if all layers are already extracted — skip network.
	if cached := loadCachedImage(imgDir); cached != nil {
		return cached, nil
	}

	var auth authn.Authenticator = authn.Anonymous
	if opts.Username != "" {
		auth = &authn.Basic{Username: opts.Username, Password: opts.Password}
	}

	// Pull linux/arm64 — the Thrive VM runs on Apple Silicon (arm64 Linux)
	platform := v1.Platform{OS: "linux", Architecture: "arm64"}
	descriptor, err := remote.Get(parsed,
		remote.WithAuth(auth),
		remote.WithContext(ctx),
		remote.WithPlatform(platform),
	)
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
		layerDir := filepath.Join(imgDir, "layers", digestHex)
		doneMarker := layerDir + "/.done"

		if _, statErr := os.Stat(doneMarker); os.IsNotExist(statErr) {
			if err := os.MkdirAll(layerDir, 0755); err != nil {
				return nil, fmt.Errorf("image.Pull: mkdir layer: %w", err)
			}
			rc, err := layer.Uncompressed()
			if err != nil {
				return nil, fmt.Errorf("image.Pull: Uncompressed[%d]: %w", i, err)
			}
			if err := extractTarDarwin(rc, layerDir); err != nil {
				rc.Close()
				return nil, fmt.Errorf("image.Pull: extract[%d]: %w", i, err)
			}
			rc.Close()
			os.WriteFile(doneMarker, []byte("done"), 0644)
		}

		imgLayers = append(imgLayers, Layer{
			Digest: digestHex,
			Size:   size,
			Path:   layerDir,
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
	if err := os.WriteFile(filepath.Join(imgDir, "manifest.json"), metaJSON, 0644); err != nil {
		return nil, fmt.Errorf("image.Pull: write manifest: %w", err)
	}

	return &Image{
		Ref:    parsed.String(),
		Digest: descriptor.Digest.String(),
		Layers: imgLayers,
	}, nil
}

// List returns all images stored in ~/.thrive/images/.
func List(ctx context.Context) ([]*Image, error) {
	imgDir := hostImageStore()
	entries, err := os.ReadDir(imgDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("image.List: %w", err)
	}

	var images []*Image
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(imgDir, entry.Name(), "manifest.json"))
		if err != nil {
			continue
		}
		var m struct {
			Ref    string
			Digest string
			Layers []Layer
		}
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		images = append(images, &Image{Ref: m.Ref, Digest: m.Digest, Layers: m.Layers})
	}
	return images, nil
}

// Remove deletes an image from the host image store.
func Remove(ctx context.Context, imageRef string) error {
	imgDir := filepath.Join(hostImageStore(), SafeRef(imageRef))
	if err := os.RemoveAll(imgDir); err != nil {
		return fmt.Errorf("image.Remove: %w", err)
	}
	return nil
}

// Push is not supported on the macOS host side.
func Push(ctx context.Context, ref string, opts PushOptions) error {
	return fmt.Errorf("push: not supported on macOS host")
}

// Mount / Unmount are no-ops on macOS — images are served via virtiofs.
func Mount(ctx context.Context, imageRef, containerID string) (string, error) {
	return "", fmt.Errorf("mount: not supported on macOS host")
}

func Unmount(ctx context.Context, containerID string) error { return nil }

func extractTarDarwin(r io.Reader, destDir string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		n := filepath.Clean(hdr.Name)
		if strings.HasPrefix(n, "..") {
			continue
		}
		target := filepath.Join(destDir, n)
		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, os.FileMode(hdr.Mode))
		case tar.TypeReg, tar.TypeRegA:
			os.MkdirAll(filepath.Dir(target), 0755)
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			io.Copy(f, tr)
			f.Close()
		case tar.TypeSymlink:
			os.MkdirAll(filepath.Dir(target), 0755)
			os.Remove(target)
			os.Symlink(hdr.Linkname, target)
		case tar.TypeLink:
			linkTarget := filepath.Join(destDir, filepath.Clean(hdr.Linkname))
			os.MkdirAll(filepath.Dir(target), 0755)
			os.Remove(target)
			os.Link(linkTarget, target)
		}
	}
	return nil
}


// loadCachedImage returns a cached Image if the manifest and all layer .done
// markers already exist on disk. Returns nil when a fresh pull is needed.
func loadCachedImage(imgDir string) *Image {
	data, err := os.ReadFile(filepath.Join(imgDir, "manifest.json"))
	if err != nil {
		return nil
	}
	var meta struct {
		Ref    string
		Digest string
		Layers []Layer
	}
	if err := json.Unmarshal(data, &meta); err != nil || len(meta.Layers) == 0 {
		return nil
	}
	for _, l := range meta.Layers {
		if _, statErr := os.Stat(l.Path + "/.done"); os.IsNotExist(statErr) {
			return nil
		}
	}
	return &Image{Ref: meta.Ref, Digest: meta.Digest, Layers: meta.Layers}
}

// ChunkStore — stub on darwin (not used on host side)
type ChunkStore struct{ basePath string }

func NewChunkStore(basePath string) *ChunkStore        { return &ChunkStore{basePath: basePath} }
func (cs *ChunkStore) Put(_ context.Context, _ string, _ []byte) error { return nil }
func (cs *ChunkStore) Get(_ context.Context, _ string) ([]byte, error) {
	return nil, fmt.Errorf("chunk store not available on macOS host")
}
func (cs *ChunkStore) Has(_ context.Context, _ string) bool { return false }
func ComputeDigest(data []byte) string                      { return "" }
