//go:build linux
// +build linux

package image

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
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

type PushOptions struct {
	Username  string
	Password  string
	PlainHTTP bool
}

// Push pushes a locally stored image to a remote registry by re-taring
// the extracted layer directories and uploading them via go-containerregistry.
func Push(ctx context.Context, ref string, opts PushOptions) error {
	log := telemetry.Logger()
	log.Info("image.Push: starting", telemetry.FieldString("ref", ref))

	parsed, err := name.ParseReference(ref)
	if err != nil {
		return fmt.Errorf("image.Push: parse reference: %w", err)
	}

	imgDir := filepath.Join("/var/lib/thrive/images", parsed.String())
	metaPath := filepath.Join(imgDir, "manifest.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return fmt.Errorf("image.Push: read manifest: %w", err)
	}

	var metadata struct {
		Layers []Layer
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("image.Push: unmarshal manifest: %w", err)
	}

	// Re-tar each extracted layer dir so go-containerregistry can consume it.
	var remLayers []v1.Layer
	for i, l := range metadata.Layers {
		layerDir := l.Path
		pr, pw := io.Pipe()

		go func(dir string) {
			tw := tar.NewWriter(pw)
			walkErr := filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				rel, _ := filepath.Rel(dir, path)
				if rel == "." {
					return nil
				}
				hdr, err := tar.FileInfoHeader(fi, "")
				if err != nil {
					return err
				}
				hdr.Name = rel
				if fi.Mode()&os.ModeSymlink != 0 {
					link, _ := os.Readlink(path)
					hdr.Linkname = link
				}
				if err := tw.WriteHeader(hdr); err != nil {
					return err
				}
				if fi.Mode().IsRegular() {
					f, err := os.Open(path)
					if err != nil {
						return err
					}
					defer f.Close()
					_, err = io.Copy(tw, f)
					return err
				}
				return nil
			})
			tw.Close()
			pw.CloseWithError(walkErr)
		}(layerDir)

		rl, err := tarball.LayerFromReader(pr)
		if err != nil {
			return fmt.Errorf("image.Push: layer[%d] LayerFromReader: %w", i, err)
		}
		remLayers = append(remLayers, rl)
	}

	img, err := mutate.AppendLayers(empty.Image, remLayers...)
	if err != nil {
		return fmt.Errorf("image.Push: AppendLayers: %w", err)
	}

	var auth authn.Authenticator = authn.Anonymous
	if opts.Username != "" {
		auth = &authn.Basic{Username: opts.Username, Password: opts.Password}
	}

	if err := remote.Write(parsed, img, remote.WithAuth(auth)); err != nil {
		return fmt.Errorf("image.Push: remote.Write: %w", err)
	}

	log.Info("image.Push: completed", telemetry.FieldString("ref", ref))
	return nil
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
		// Extract layer into its own directory so OverlayFS can use it as a lowerdir.
		layerDir := filepath.Join(imgDir, "layers", digestHex)
		doneMarker := layerDir + "/.done"
		log.Info("image.Pull: layer dir", telemetry.FieldInt("layer", i), telemetry.FieldString("digest", digestHex[:12]), telemetry.FieldString("path", layerDir))

		if _, err := os.Stat(doneMarker); os.IsNotExist(err) {
			log.Info("image.Pull: extracting layer", telemetry.FieldInt("layer", i), telemetry.FieldInt64("size", size))

			if err := os.MkdirAll(layerDir, 0755); err != nil {
				log.Error("image.Pull: mkdir layer dir failed", telemetry.FieldString("path", layerDir), telemetry.FieldError(err))
				return nil, fmt.Errorf("image.Pull: mkdir layer: %w", err)
			}

			rc, err := layer.Uncompressed()
			if err != nil {
				log.Error("image.Pull: layer.Uncompressed failed", telemetry.FieldInt("layer", i), telemetry.FieldError(err))
				return nil, fmt.Errorf("image.Pull: layer[%d] Uncompressed: %w", i, err)
			}

			if err := extractTar(rc, layerDir); err != nil {
				rc.Close()
				log.Error("image.Pull: extractTar failed", telemetry.FieldInt("layer", i), telemetry.FieldError(err))
				return nil, fmt.Errorf("image.Pull: extract layer[%d]: %w", i, err)
			}
			rc.Close()

			// Mark layer as fully extracted.
			os.WriteFile(doneMarker, []byte("done"), 0644)
			telemetry.Debug("image.Pull: layer extracted", telemetry.FieldInt("layer", i))
		} else {
			telemetry.Debug("image.Pull: layer already extracted", telemetry.FieldInt("layer", i), telemetry.FieldString("digest", digestHex[:12]))
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

	log.Info("image.Pull: saving manifest", telemetry.FieldString("ref", parsed.String()))
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		log.Error("image.Pull: marshal manifest failed", telemetry.FieldError(err))
		return nil, fmt.Errorf("image.Pull: marshal manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(imgDir, "manifest.json"), metaJSON, 0644); err != nil {
		log.Error("image.Pull: write manifest failed", telemetry.FieldError(err))
		return nil, fmt.Errorf("image.Pull: write manifest: %w", err)
	}

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
	metaPath := filepath.Join(imgDir, "manifest.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		log.Error("image.Mount: read manifest failed", telemetry.FieldString("path", metaPath), telemetry.FieldError(err))
		return "", fmt.Errorf("image.Mount: read manifest: %w", err)
	}

	var metadata struct {
		Layers []Layer
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		log.Error("image.Mount: unmarshal manifest failed", telemetry.FieldError(err))
		return "", fmt.Errorf("image.Mount: unmarshal: %w", err)
	}

	if len(metadata.Layers) == 0 {
		return "", fmt.Errorf("image.Mount: no layers found for %s", imageRef)
	}

	containerDir := filepath.Join("/run/thrive/containers", containerID)
	upperDir := filepath.Join(containerDir, "upper")
	workDir := filepath.Join(containerDir, "work")
	mergedDir := filepath.Join(containerDir, "merged")

	for _, d := range []string{upperDir, workDir, mergedDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return "", fmt.Errorf("image.Mount: mkdir %s: %w", d, err)
		}
	}

	// OverlayFS requires layers in bottom-to-top order for lowerdir (lowest first).
	lowerParts := make([]string, len(metadata.Layers))
	for i, l := range metadata.Layers {
		lowerParts[len(metadata.Layers)-1-i] = l.Path
	}
	lowerDirs := strings.Join(lowerParts, ":")

	overlayOpts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDirs, upperDir, workDir)

	// Try kernel overlay mount first (requires root or user namespace with overlay).
	mountErr := syscall.Mount("overlay", mergedDir, "overlay", 0, overlayOpts)
	if mountErr == nil {
		log.Info("image.Mount: overlay mounted", telemetry.FieldString("merged", mergedDir))
		return mergedDir, nil
	}
	log.Info("image.Mount: kernel overlay failed, trying fuse-overlayfs", telemetry.FieldError(mountErr))

	// Rootless fallback: fuse-overlayfs.
	fuseArgs := []string{
		"-o", overlayOpts,
		mergedDir,
	}
	out, fuseErr := exec.CommandContext(ctx, "fuse-overlayfs", fuseArgs...).CombinedOutput()
	if fuseErr != nil {
		log.Error("image.Mount: fuse-overlayfs failed", telemetry.FieldString("output", string(out)), telemetry.FieldError(fuseErr))
		return "", fmt.Errorf("image.Mount: overlay mount failed (kernel: %v, fuse: %v)", mountErr, fuseErr)
	}

	log.Info("image.Mount: fuse-overlayfs mounted", telemetry.FieldString("merged", mergedDir))
	return mergedDir, nil
}

func Unmount(ctx context.Context, containerID string) error {
	log := telemetry.Logger()
	mergedDir := filepath.Join("/run/thrive/containers", containerID, "merged")
	log.Info("image.Unmount: starting", telemetry.FieldString("mergedDir", mergedDir))

	if err := syscall.Unmount(mergedDir, syscall.MNT_DETACH); err != nil && err != syscall.EINVAL {
		log.Error("image.Unmount: failed", telemetry.FieldError(err))
		return fmt.Errorf("image.Unmount: %w", err)
	}
	log.Info("image.Unmount: completed", telemetry.FieldString("containerID", containerID))
	return nil
}

// extractTar extracts a tar stream into destDir, handling files, dirs, and symlinks.
func extractTar(r io.Reader, destDir string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("extractTar: read header: %w", err)
		}

		// Strip leading "./" or "/" from names to prevent path traversal.
		name := filepath.Clean(hdr.Name)
		if strings.HasPrefix(name, "..") {
			continue
		}
		target := filepath.Join(destDir, name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return fmt.Errorf("extractTar: mkdir %s: %w", target, err)
			}

		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("extractTar: mkdir parent %s: %w", filepath.Dir(target), err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return fmt.Errorf("extractTar: create %s: %w", target, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("extractTar: write %s: %w", target, err)
			}
			f.Close()

		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("extractTar: mkdir parent %s: %w", filepath.Dir(target), err)
			}
			os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return fmt.Errorf("extractTar: symlink %s: %w", target, err)
			}

		case tar.TypeLink:
			linkTarget := filepath.Join(destDir, filepath.Clean(hdr.Linkname))
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("extractTar: mkdir parent %s: %w", filepath.Dir(target), err)
			}
			os.Remove(target)
			if err := os.Link(linkTarget, target); err != nil {
				return fmt.Errorf("extractTar: hardlink %s: %w", target, err)
			}
		}
	}
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
