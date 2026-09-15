package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/thakurprasadrout/thrive/internal/image"
)

// LayerRecord describes one image layer for `thrive history`.
type LayerRecord struct {
	Index  int    `json:"index"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

// HistoryTo reads the stored manifest for ref in storeDir.
func HistoryTo(ctx context.Context, storeDir, ref string) ([]LayerRecord, error) {
	data, err := os.ReadFile(filepath.Join(storeDir, image.SafeRef(ref), "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("registry: history: image %q not found: %w", ref, err)
	}
	var meta struct {
		Digest string
		Layers []image.Layer
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("registry: history: parse manifest: %w", err)
	}
	out := make([]LayerRecord, 0, len(meta.Layers))
	for i, l := range meta.Layers {
		out = append(out, LayerRecord{Index: i, Digest: l.Digest, Size: l.Size})
	}
	return out, nil
}

// ImportTo extracts a filesystem tarball from r as a new single-layer
// image named newRef in storeDir (docker import parity).
func ImportTo(ctx context.Context, storeDir string, r io.Reader, newRef string) error {
	if newRef == "" {
		return fmt.Errorf("registry: import requires an image reference")
	}
	sum := sha256.Sum256([]byte(newRef + time.Now().UTC().String()))
	digest := "sha256:" + hex.EncodeToString(sum[:])
	imgDir := filepath.Join(storeDir, image.SafeRef(newRef))
	layerDir := filepath.Join(imgDir, "layers", digest)
	if err := os.MkdirAll(layerDir, 0755); err != nil {
		return fmt.Errorf("registry: import: mkdir: %w", err)
	}
	if err := extractTar(r, layerDir); err != nil {
		os.RemoveAll(imgDir) //nolint:errcheck
		return fmt.Errorf("registry: import: extract: %w", err)
	}
	os.WriteFile(layerDir+"/.done", []byte("done"), 0644) //nolint:errcheck
	meta := struct {
		Ref    string
		Digest string
		Layers []image.Layer
	}{
		Ref:    newRef,
		Digest: digest,
		Layers: []image.Layer{{Digest: digest, Path: layerDir}},
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("registry: import: marshal: %w", err)
	}
	if err := os.WriteFile(filepath.Join(imgDir, "manifest.json"), metaJSON, 0644); err != nil {
		return fmt.Errorf("registry: import: write manifest: %w", err)
	}
	return nil
}
