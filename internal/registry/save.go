package registry

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/thakurprasadrout/thrive/internal/image"
)

// SaveArchiveVersion is written into thrive-save.json so future
// format changes can be detected on load.
const SaveArchiveVersion = 1

type savedImageEntry struct {
	Ref      string   `json:"ref"`
	Digest   string   `json:"digest"`
	Manifest string   `json:"manifest"`
	Layers   []string `json:"layers"`
}

type saveIndex struct {
	Version int               `json:"version"`
	Images  []savedImageEntry `json:"images"`
}

// SaveTo writes refs from storeDir as a thrive-native tar archive to w.
// Layout: thrive-save.json + manifests/<safe>.json + layers/<digest>.tar.
func SaveTo(ctx context.Context, storeDir string, refs []string, w io.Writer) error {
	if len(refs) == 0 {
		return fmt.Errorf("registry: save requires at least one image")
	}
	tw := tar.NewWriter(w)
	defer tw.Close() //nolint:errcheck

	index := saveIndex{Version: SaveArchiveVersion}
	for _, ref := range refs {
		manifestPath := filepath.Join(storeDir, image.SafeRef(ref), "manifest.json")
		metaJSON, err := os.ReadFile(manifestPath)
		if err != nil {
			return fmt.Errorf("registry: save: image %q not found: %w", ref, err)
		}
		var meta struct {
			Ref    string
			Digest string
			Layers []image.Layer
		}
		if err := json.Unmarshal(metaJSON, &meta); err != nil {
			return fmt.Errorf("registry: save: parse manifest for %q: %w", ref, err)
		}
		entry := savedImageEntry{
			Ref:      ref,
			Digest:   meta.Digest,
			Manifest: "manifests/" + image.SafeRef(ref) + ".json",
		}
		if err := writeTarFile(tw, entry.Manifest, metaJSON, 0644); err != nil {
			return fmt.Errorf("registry: save: %w", err)
		}
		for _, l := range meta.Layers {
			layerTar := "layers/" + l.Digest + ".tar"
			layerDir := filepath.Join(storeDir, image.SafeRef(ref), "layers", l.Digest)
			if _, err := os.Stat(layerDir); err != nil {
				return fmt.Errorf("registry: save: layer %s missing for %q: %w", l.Digest, ref, err)
			}
			pr, pw := io.Pipe()
			go func(dir string, w *io.PipeWriter) {
				w.CloseWithError(tarDirToWriter(dir, w)) //nolint:errcheck
			}(layerDir, pw)
			if err := writeTarStream(tw, layerTar, pr); err != nil {
				return fmt.Errorf("registry: save: layer %s: %w", l.Digest, err)
			}
			entry.Layers = append(entry.Layers, layerTar)
		}
		index.Images = append(index.Images, entry)
	}
	indexJSON, err := json.Marshal(index)
	if err != nil {
		return fmt.Errorf("registry: save: marshal index: %w", err)
	}
	if err := writeTarFile(tw, "thrive-save.json", indexJSON, 0644); err != nil {
		return fmt.Errorf("registry: save: %w", err)
	}
	return nil
}

// LoadFrom reads a thrive-native save archive from r into storeDir,
// returning the loaded image refs.
func LoadFrom(ctx context.Context, storeDir string, r io.Reader) ([]string, error) {
	tmpDir, err := os.MkdirTemp("", "thrive-load-*")
	if err != nil {
		return nil, fmt.Errorf("registry: load: tempdir: %w", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck

	if err := extractTar(r, tmpDir); err != nil {
		return nil, fmt.Errorf("registry: load: extract: %w", err)
	}
	indexJSON, err := os.ReadFile(filepath.Join(tmpDir, "thrive-save.json"))
	if err != nil {
		return nil, fmt.Errorf("registry: load: not a thrive save archive (missing thrive-save.json): %w", err)
	}
	var index saveIndex
	if err := json.Unmarshal(indexJSON, &index); err != nil {
		return nil, fmt.Errorf("registry: load: parse index: %w", err)
	}
	if index.Version != SaveArchiveVersion {
		return nil, fmt.Errorf("registry: load: unsupported archive version %d", index.Version)
	}
	var loaded []string
	for _, entry := range index.Images {
		metaJSON, err := os.ReadFile(filepath.Join(tmpDir, entry.Manifest))
		if err != nil {
			return nil, fmt.Errorf("registry: load: manifest for %q: %w", entry.Ref, err)
		}
		var meta struct {
			Ref    string
			Digest string
			Layers []image.Layer
		}
		if err := json.Unmarshal(metaJSON, &meta); err != nil {
			return nil, fmt.Errorf("registry: load: parse manifest for %q: %w", entry.Ref, err)
		}
		imgDir := filepath.Join(storeDir, image.SafeRef(entry.Ref))
		if err := os.MkdirAll(imgDir, 0755); err != nil {
			return nil, fmt.Errorf("registry: load: mkdir: %w", err)
		}
		if len(entry.Layers) != len(meta.Layers) {
			return nil, fmt.Errorf("registry: load: layer count mismatch for %q", entry.Ref)
		}
		// Rewrite layer paths to this store and re-extract layer tars.
		for i, layerTar := range entry.Layers {
			layerDir := filepath.Join(imgDir, "layers", meta.Layers[i].Digest)
			if err := os.MkdirAll(layerDir, 0755); err != nil {
				return nil, fmt.Errorf("registry: load: mkdir layer: %w", err)
			}
			f, err := os.Open(filepath.Join(tmpDir, layerTar))
			if err != nil {
				return nil, fmt.Errorf("registry: load: open %s: %w", layerTar, err)
			}
			err = extractTar(f, layerDir)
			_ = f.Close()
			if err != nil {
				return nil, fmt.Errorf("registry: load: extract %s: %w", layerTar, err)
			}
			os.WriteFile(layerDir+"/.done", []byte("done"), 0644) //nolint:errcheck
			meta.Layers[i].Path = layerDir
		}
		updated, err := json.Marshal(meta)
		if err != nil {
			return nil, fmt.Errorf("registry: load: marshal: %w", err)
		}
		if err := os.WriteFile(filepath.Join(imgDir, "manifest.json"), updated, 0644); err != nil {
			return nil, fmt.Errorf("registry: load: write manifest: %w", err)
		}
		loaded = append(loaded, entry.Ref)
	}
	return loaded, nil
}

// TarDirectory tars srcDir to w (public wrapper for cp/export paths).
func TarDirectory(srcDir string, w io.Writer) error {
	return tarDirToWriter(srcDir, w)
}

// ExtractArchive extracts a tar stream into dst (public wrapper).
func ExtractArchive(r io.Reader, dst string) error {
	return extractTar(r, dst)
}

func writeTarFile(tw *tar.Writer, name string, data []byte, mode int64) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(data))}); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func writeTarStream(tw *tar.Writer, name string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return writeTarFile(tw, name, data, 0644)
}

func tarDirToWriter(srcDir string, w io.Writer) error {
	tw := tar.NewWriter(w)
	defer tw.Close() //nolint:errcheck
	return filepath.Walk(srcDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(srcDir, path)
		if rel == "." || rel == ".done" {
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
			_, err = io.Copy(tw, f)
			cerr := f.Close()
			if err != nil {
				return err
			}
			return cerr
		}
		return nil
	})
}

func extractTar(r io.Reader, destDir string) error {
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
		if n == "." || n == ".." || len(n) > 2 && (n[:3] == "../" || n == "..") {
			continue
		}
		target := filepath.Join(destDir, n)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			_, err = io.Copy(f, tr)
			f.Close()
			if err != nil {
				return err
			}
		case tar.TypeSymlink:
			os.MkdirAll(filepath.Dir(target), 0755) //nolint:errcheck
			os.Remove(target)                       //nolint:errcheck
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		case tar.TypeLink:
			linkTarget := filepath.Join(destDir, filepath.Clean(hdr.Linkname))
			os.MkdirAll(filepath.Dir(target), 0755) //nolint:errcheck
			os.Remove(target)                       //nolint:errcheck
			if err := os.Link(linkTarget, target); err != nil {
				return err
			}
		}
	}
	return nil
}
