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
	"github.com/thakurprasadrout/thrive/internal/telemetry"
)

type Image struct {
	Ref    string
	Digest string
	Layers []Layer
}

type Layer struct {
	Digest string
	Size   int64
	Path   string
}

type PullOptions struct {
	Username  string
	Password  string
	PlainHTTP bool
}

func Pull(ctx context.Context, ref string, opts PullOptions) (*Image, error) {
	log := telemetry.Logger()
	log.Info("image.Pull: starting", telemetry.FieldString("ref", ref))

	parsed, err := name.ParseReference(ref)
	if err != nil {
		log.Error("image.Pull: ParseReference failed", telemetry.FieldString("ref", ref), telemetry.FieldError(err))
		return nil, fmt.Errorf("image.Pull: parse reference %s: %w", ref, err)
	}
	telemetry.Debug("image.Pull: reference parsed", telemetry.FieldString("name", parsed.Name()))

	imgDir := filepath.Join("/var/lib/thrive/images", parsed.String())
	log.Info("image.Pull: creating image directory", telemetry.FieldString("path", imgDir))

	if err := os.MkdirAll(imgDir, 0755); err != nil {
		log.Error("image.Pull: mkdir failed", telemetry.FieldString("path", imgDir), telemetry.FieldError(err))
		return nil, fmt.Errorf("image.Pull: mkdir %s: %w", imgDir, err)
	}
	telemetry.Debug("image.Pull: image directory created", telemetry.FieldString("path", imgDir))

	var auth authn.Authenticator
	if opts.Username != "" {
		auth = &authn.Basic{Username: opts.Username, Password: opts.Password}
		log.Info("image.Pull: using authentication", telemetry.FieldString("username", opts.Username))
	}
	telemetry.Debug("image.Pull: auth configured", telemetry.FieldBool("hasAuth", auth != nil))

	log.Info("image.Pull: fetching image from registry", telemetry.FieldString("ref", ref))
	descriptor, err := remote.Get(parsed, remote.WithAuth(auth))
	if err != nil {
		log.Error("image.Pull: remote.Get failed", telemetry.FieldString("ref", ref), telemetry.FieldError(err))
		return nil, fmt.Errorf("image.Pull: remote.Get: %w", err)
	}
	telemetry.Debug("image.Pull: image descriptor received", telemetry.FieldString("digest", descriptor.Digest.String()))

	log.Info("image.Pull: getting image from descriptor")
	img, err := descriptor.Image()
	if err != nil {
		log.Error("image.Pull: descriptor.Image failed", telemetry.FieldError(err))
		return nil, fmt.Errorf("image.Pull: Image: %w", err)
	}
	telemetry.Debug("image.Pull: image obtained")

	log.Info("image.Pull: extracting layers")
	layers, err := img.Layers()
	if err != nil {
		log.Error("image.Pull: img.Layers failed", telemetry.FieldError(err))
		return nil, fmt.Errorf("image.Pull: Layers: %w", err)
	}
	telemetry.Debug("image.Pull: layers extracted", telemetry.FieldInt("count", len(layers)))

	var imgLayers []Layer
	for i, layer := range layers {
		log.Info("image.Pull: processing layer", telemetry.FieldInt("index", i), telemetry.FieldInt("total", len(layers)))

		digest, err := layer.Digest()
		if err != nil {
			log.Error("image.Pull: layer.Digest failed", telemetry.FieldInt("layer", i), telemetry.FieldError(err))
			return nil, fmt.Errorf("image.Pull: layer[%d] Digest: %w", i, err)
		}

		size, err := layer.Size()
		if err != nil {
			log.Error("image.Pull: layer.Size failed", telemetry.FieldInt("layer", i), telemetry.FieldError(err))
			return nil, fmt.Errorf("image.Pull: layer[%d] Size: %w", i, err)
		}

		digestHex := digest.String()
		chunkPath := filepath.Join("/var/lib/thrive/chunks", digestHex[:2], digestHex[2:])
		log.Info("image.Pull: chunk path computed", telemetry.FieldInt("layer", i), telemetry.FieldString("digest", digestHex[:12]), telemetry.FieldString("path", chunkPath))

		if err := os.MkdirAll(filepath.Dir(chunkPath), 0755); err != nil {
			log.Error("image.Pull: mkdir chunk dir failed", telemetry.FieldString("path", filepath.Dir(chunkPath)), telemetry.FieldError(err))
			return nil, fmt.Errorf("image.Pull: mkdir chunk: %w", err)
		}

		if _, err := os.Stat(chunkPath); os.IsNotExist(err) {
			log.Info("image.Pull: downloading layer", telemetry.FieldInt("layer", i), telemetry.FieldInt64("size", size))

			rc, err := layer.Uncompressed()
			if err != nil {
				log.Error("image.Pull: layer.Uncompressed failed", telemetry.FieldInt("layer", i), telemetry.FieldError(err))
				return nil, fmt.Errorf("image.Pull: layer[%d] Uncompressed: %w", i, err)
			}

			f, err := os.Create(chunkPath)
			if err != nil {
				rc.Close()
				log.Error("image.Pull: create chunk file failed", telemetry.FieldString("path", chunkPath), telemetry.FieldError(err))
				return nil, fmt.Errorf("image.Pull: create chunk: %w", err)
			}

			written, err := io.Copy(f, rc)
			rc.Close()
			f.Close()

			if err != nil {
				log.Error("image.Pull: io.Copy failed", telemetry.FieldInt("layer", i), telemetry.FieldError(err))
				return nil, fmt.Errorf("image.Pull: copy layer: %w", err)
			}
			telemetry.Debug("image.Pull: layer downloaded", telemetry.FieldInt("layer", i), telemetry.FieldInt64("written", written))
		} else {
			telemetry.Debug("image.Pull: layer already cached", telemetry.FieldInt("layer", i), telemetry.FieldString("digest", digestHex[:12]))
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

	log.Info("image.Pull: saving manifest", telemetry.FieldString("ref", parsed.String()))
	metaJSON, _ := json.Marshal(metadata)
	os.WriteFile(filepath.Join(imgDir, "manifest.json"), metaJSON, 0644)

	log.Info("image.Pull: completed successfully", telemetry.FieldString("ref", parsed.String()), telemetry.FieldString("digest", descriptor.Digest.String()[:12]), telemetry.FieldInt("layers", len(imgLayers)))

	return &Image{
		Ref:    parsed.String(),
		Digest: descriptor.Digest.String(),
		Layers: imgLayers,
	}, nil
}

func Mount(ctx context.Context, imageRef, containerID string) (string, error) {
	log := telemetry.Logger()
	log.Info("image.Mount: starting", telemetry.FieldString("imageRef", imageRef), telemetry.FieldString("containerID", containerID))

	imgDir := filepath.Join("/var/lib/thrive/images", imageRef)
	log.Info("image.Mount: reading manifest", telemetry.FieldString("path", imgDir))

	metaPath := filepath.Join(imgDir, "manifest.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		log.Error("image.Mount: read manifest failed", telemetry.FieldString("path", metaPath), telemetry.FieldError(err))
		return "", fmt.Errorf("image.Mount: read manifest: %w", err)
	}
	telemetry.Debug("image.Mount: manifest read", telemetry.FieldInt("size", len(data)))

	var metadata struct {
		Layers []Layer
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		log.Error("image.Mount: unmarshal manifest failed", telemetry.FieldError(err))
		return "", fmt.Errorf("image.Mount: unmarshal: %w", err)
	}
	telemetry.Debug("image.Mount: manifest unmarshaled", telemetry.FieldInt("layers", len(metadata.Layers)))

	containerDir := filepath.Join("/run/thrive/containers", containerID)
	upperDir := filepath.Join(containerDir, "upper")
	workDir := filepath.Join(containerDir, "work")
	telemetry.Debug("image.Mount: creating container directories", telemetry.FieldString("containerDir", containerDir))

	os.MkdirAll(upperDir, 0755)
	os.MkdirAll(workDir, 0755)

	if len(metadata.Layers) > 0 {
		log.Info("image.Mount: returning first layer path", telemetry.FieldString("path", metadata.Layers[0].Path))
		return metadata.Layers[0].Path, nil
	}

	log.Error("image.Mount: no layers found", telemetry.FieldString("imageRef", imageRef))
	return "", fmt.Errorf("image.Mount: no layers found")
}

func Unmount(ctx context.Context, containerID string) error {
	log := telemetry.Logger()
	log.Info("image.Unmount: starting", telemetry.FieldString("containerID", containerID))
	log.Info("image.Unmount: completed (stub)")
	return nil
}

func List(ctx context.Context) ([]*Image, error) {
	log := telemetry.Logger()
	log.Info("image.List: starting")

	imgDir := "/var/lib/thrive/images"
	entries, err := os.ReadDir(imgDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Info("image.List: image directory does not exist")
			return nil, nil
		}
		log.Error("image.List: ReadDir failed", telemetry.FieldString("path", imgDir), telemetry.FieldError(err))
		return nil, fmt.Errorf("image.List: readdir: %w", err)
	}
	telemetry.Debug("image.List: directory entries scanned", telemetry.FieldInt("count", len(entries)))

	var images []*Image
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(imgDir, entry.Name(), "manifest.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			telemetry.Debug("image.List: skipping entry (no manifest)", telemetry.FieldString("entry", entry.Name()))
			continue
		}
		var metadata struct {
			Ref    string
			Digest string
			Layers []Layer
		}
		if err := json.Unmarshal(data, &metadata); err != nil {
			telemetry.Debug("image.List: skipping entry (invalid manifest)", telemetry.FieldString("entry", entry.Name()))
			continue
		}
		images = append(images, &Image{
			Ref:    metadata.Ref,
			Digest: metadata.Digest,
			Layers: metadata.Layers,
		})
		telemetry.Debug("image.List: image found", telemetry.FieldString("ref", metadata.Ref))
	}

	log.Info("image.List: completed", telemetry.FieldInt("count", len(images)))
	return images, nil
}

func Remove(ctx context.Context, imageRef string) error {
	log := telemetry.Logger()
	log.Info("image.Remove: starting", telemetry.FieldString("imageRef", imageRef))

	imgDir := filepath.Join("/var/lib/thrive/images", imageRef)
	log.Info("image.Remove: removing image directory", telemetry.FieldString("path", imgDir))

	if err := os.RemoveAll(imgDir); err != nil {
		log.Error("image.Remove: RemoveAll failed", telemetry.FieldString("path", imgDir), telemetry.FieldError(err))
		return err
	}

	log.Info("image.Remove: completed successfully", telemetry.FieldString("imageRef", imageRef))
	return nil
}

type ChunkStore struct {
	basePath string
}

func NewChunkStore(basePath string) *ChunkStore {
	return &ChunkStore{basePath: basePath}
}

func (cs *ChunkStore) Put(ctx context.Context, digest string, data []byte) error {
	log := telemetry.Logger()
	log.Debug("ChunkStore.Put: starting", telemetry.FieldString("digest", digest[:12]), telemetry.FieldInt("size", len(data)))

	chunkPath := cs.chunkPath(digest)
	if err := os.MkdirAll(filepath.Dir(chunkPath), 0755); err != nil {
		log.Error("ChunkStore.Put: mkdir failed", telemetry.FieldString("path", filepath.Dir(chunkPath)), telemetry.FieldError(err))
		return fmt.Errorf("ChunkStore.Put: mkdir: %w", err)
	}
	if err := os.WriteFile(chunkPath, data, 0644); err != nil {
		log.Error("ChunkStore.Put: WriteFile failed", telemetry.FieldString("path", chunkPath), telemetry.FieldError(err))
		return err
	}
	log.Debug("ChunkStore.Put: completed", telemetry.FieldString("digest", digest[:12]))
	return nil
}

func (cs *ChunkStore) Get(ctx context.Context, digest string) ([]byte, error) {
	log := telemetry.Logger()
	log.Debug("ChunkStore.Get: starting", telemetry.FieldString("digest", digest[:12]))

	data, err := os.ReadFile(cs.chunkPath(digest))
	if err != nil {
		log.Error("ChunkStore.Get: ReadFile failed", telemetry.FieldString("path", cs.chunkPath(digest)), telemetry.FieldError(err))
		return nil, err
	}
	log.Debug("ChunkStore.Get: completed", telemetry.FieldString("digest", digest[:12]), telemetry.FieldInt("size", len(data)))
	return data, nil
}

func (cs *ChunkStore) Has(ctx context.Context, digest string) bool {
	_, err := os.Stat(cs.chunkPath(digest))
	return err == nil
}

func (cs *ChunkStore) chunkPath(digest string) string {
	return filepath.Join(cs.basePath, digest[:2], digest[2:])
}

func ComputeDigest(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
