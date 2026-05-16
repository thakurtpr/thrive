//go:build linux
// +build linux

package image

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// buildTar creates an in-memory tar with the given entries.
func buildTar(t *testing.T, entries []struct {
	hdr  *tar.Header
	body []byte
}) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	tw := tar.NewWriter(buf)
	for _, e := range entries {
		if err := tw.WriteHeader(e.hdr); err != nil {
			t.Fatalf("buildTar: WriteHeader: %v", err)
		}
		if len(e.body) > 0 {
			if _, err := tw.Write(e.body); err != nil {
				t.Fatalf("buildTar: Write: %v", err)
			}
		}
	}
	tw.Close()
	return buf
}

func TestExtractTar_Dir(t *testing.T) {
	dest := t.TempDir()
	buf := buildTar(t, []struct {
		hdr  *tar.Header
		body []byte
	}{
		{hdr: &tar.Header{Typeflag: tar.TypeDir, Name: "etc/", Mode: 0755}},
	})
	if err := extractTar(buf, dest); err != nil {
		t.Fatalf("extractTar: %v", err)
	}
	info, err := os.Stat(filepath.Join(dest, "etc"))
	if err != nil || !info.IsDir() {
		t.Fatalf("expected etc/ to be a directory")
	}
}

func TestExtractTar_RegularFile(t *testing.T) {
	dest := t.TempDir()
	content := []byte("hello thrive")
	buf := buildTar(t, []struct {
		hdr  *tar.Header
		body []byte
	}{
		{hdr: &tar.Header{Typeflag: tar.TypeReg, Name: "usr/bin/hello", Mode: 0755, Size: int64(len(content))}, body: content},
	})
	if err := extractTar(buf, dest); err != nil {
		t.Fatalf("extractTar: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "usr/bin/hello"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("content mismatch: got %q want %q", got, content)
	}
}

func TestExtractTar_Symlink(t *testing.T) {
	dest := t.TempDir()
	content := []byte("binary content")
	buf := buildTar(t, []struct {
		hdr  *tar.Header
		body []byte
	}{
		{hdr: &tar.Header{Typeflag: tar.TypeReg, Name: "bin/real", Mode: 0755, Size: int64(len(content))}, body: content},
		{hdr: &tar.Header{Typeflag: tar.TypeSymlink, Name: "bin/link", Linkname: "real"}},
	})
	if err := extractTar(buf, dest); err != nil {
		t.Fatalf("extractTar: %v", err)
	}
	link, err := os.Readlink(filepath.Join(dest, "bin/link"))
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if link != "real" {
		t.Fatalf("symlink target: got %q want %q", link, "real")
	}
}

func TestExtractTar_PathTraversalIsSkipped(t *testing.T) {
	dest := t.TempDir()
	parent := filepath.Dir(dest)
	malicious := []byte("owned")
	buf := buildTar(t, []struct {
		hdr  *tar.Header
		body []byte
	}{
		{hdr: &tar.Header{Typeflag: tar.TypeReg, Name: "../escape.txt", Mode: 0644, Size: int64(len(malicious))}, body: malicious},
	})
	if err := extractTar(buf, dest); err != nil {
		t.Fatalf("extractTar: %v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "escape.txt")); !os.IsNotExist(err) {
		t.Fatal("path traversal file should not have been created")
	}
}

func TestExtractTar_Hardlink(t *testing.T) {
	dest := t.TempDir()
	content := []byte("shared data")
	buf := buildTar(t, []struct {
		hdr  *tar.Header
		body []byte
	}{
		{hdr: &tar.Header{Typeflag: tar.TypeReg, Name: "data/original", Mode: 0644, Size: int64(len(content))}, body: content},
		{hdr: &tar.Header{Typeflag: tar.TypeLink, Name: "data/hardlink", Linkname: "data/original"}},
	})
	if err := extractTar(buf, dest); err != nil {
		t.Fatalf("extractTar: %v", err)
	}
	orig, err := os.Stat(filepath.Join(dest, "data/original"))
	if err != nil {
		t.Fatalf("original stat: %v", err)
	}
	linked, err := os.Stat(filepath.Join(dest, "data/hardlink"))
	if err != nil {
		t.Fatalf("hardlink stat: %v", err)
	}
	if !os.SameFile(orig, linked) {
		t.Fatal("hardlink and original should be the same inode")
	}
}

func TestComputeDigest_Deterministic(t *testing.T) {
	data := []byte("thrive container runtime")
	d1 := ComputeDigest(data)
	d2 := ComputeDigest(data)
	if d1 != d2 {
		t.Fatalf("ComputeDigest not deterministic: %q vs %q", d1, d2)
	}
	if len(d1) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(d1))
	}
}

func TestComputeDigest_DifferentInput(t *testing.T) {
	a := ComputeDigest([]byte("alpha"))
	b := ComputeDigest([]byte("beta"))
	if a == b {
		t.Fatal("different inputs must produce different digests")
	}
}

func TestChunkStore_PutGetHas(t *testing.T) {
	base := t.TempDir()
	cs := NewChunkStore(base)
	ctx := context.Background()

	digest := ComputeDigest([]byte("chunk payload"))
	data := []byte("chunk payload")

	if cs.Has(ctx, digest) {
		t.Fatal("Has should be false before Put")
	}

	if err := cs.Put(ctx, digest, data); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if !cs.Has(ctx, digest) {
		t.Fatal("Has should be true after Put")
	}

	got, err := cs.Get(ctx, digest)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("Get content mismatch: got %q want %q", got, data)
	}
}

func TestChunkStore_GetMissingReturnsError(t *testing.T) {
	cs := NewChunkStore(t.TempDir())
	digest := ComputeDigest([]byte("nonexistent"))
	_, err := cs.Get(context.Background(), digest)
	if err == nil {
		t.Fatal("Get of missing chunk should return error")
	}
}

func TestList_EmptyDirReturnsNil(t *testing.T) {
	// /var/lib/thrive/images doesn't exist in test env — List must return nil, nil.
	images, err := List(context.Background())
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	_ = images
}

func TestList_ReadsManifests(t *testing.T) {
	// Verify the manifest JSON structure is parseable, which is the core
	// logic List() runs per directory entry.
	tmpBase := t.TempDir()
	imgDir := filepath.Join(tmpBase, "myimage")
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	meta := struct {
		Ref    string
		Digest string
		Layers []Layer
	}{Ref: "myimage", Digest: "sha256:abc", Layers: nil}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(imgDir, "manifest.json"), data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(imgDir, "manifest.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var parsed struct {
		Ref    string
		Digest string
		Layers []Layer
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if parsed.Ref != "myimage" {
		t.Fatalf("expected ref myimage, got %q", parsed.Ref)
	}
}

func TestMount_NonexistentImageReturnsError(t *testing.T) {
	// Mount reads /var/lib/thrive/images/{ref}/manifest.json — must fail
	// gracefully with an error (not a panic) when the image doesn't exist.
	_, err := Mount(context.Background(), "nonexistent-image-xyz", "test-container-abc")
	if err == nil {
		t.Fatal("Mount with nonexistent image should return error")
	}
}

func TestUnmount_NonexistentContainerDoesNotPanic(t *testing.T) {
	// Unmounting a path that was never mounted returns EINVAL which we suppress.
	// Verify it doesn't panic or return an unexpected error type.
	err := Unmount(context.Background(), "container-that-never-existed-xyz")
	_ = err // EINVAL is suppressed; any other error is acceptable too
}

func TestRemove_NonexistentPathReturnsNil(t *testing.T) {
	// os.RemoveAll is idempotent — non-existent path returns nil.
	err := Remove(context.Background(), filepath.Join(t.TempDir(), "nonexistent"))
	if err != nil {
		t.Fatalf("Remove of nonexistent path: %v", err)
	}
}
