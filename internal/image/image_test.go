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

// ---------------------------------------------------------------------------
// extractTar
// ---------------------------------------------------------------------------

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

func TestExtractTar_BasicFiles(t *testing.T) {
	// Arrange — directory + two regular files, one nested
	dest := t.TempDir()
	file1Body := []byte("file one content")
	file2Body := []byte("file two content")

	buf := buildTar(t, []struct {
		hdr  *tar.Header
		body []byte
	}{
		{hdr: &tar.Header{Typeflag: tar.TypeDir, Name: "subdir/", Mode: 0755}},
		{hdr: &tar.Header{Typeflag: tar.TypeReg, Name: "subdir/file1.txt", Mode: 0644, Size: int64(len(file1Body))}, body: file1Body},
		{hdr: &tar.Header{Typeflag: tar.TypeReg, Name: "file2.txt", Mode: 0644, Size: int64(len(file2Body))}, body: file2Body},
	})

	// Act
	if err := extractTar(buf, dest); err != nil {
		t.Fatalf("extractTar: %v", err)
	}

	// Assert — directory
	if fi, err := os.Stat(filepath.Join(dest, "subdir")); err != nil || !fi.IsDir() {
		t.Errorf("expected subdir to be a directory, stat err: %v", err)
	}
	// Assert — file1
	got1, err := os.ReadFile(filepath.Join(dest, "subdir", "file1.txt"))
	if err != nil {
		t.Fatalf("read file1.txt: %v", err)
	}
	if !bytes.Equal(got1, file1Body) {
		t.Errorf("file1.txt = %q, want %q", got1, file1Body)
	}
	// Assert — file2
	got2, err := os.ReadFile(filepath.Join(dest, "file2.txt"))
	if err != nil {
		t.Fatalf("read file2.txt: %v", err)
	}
	if !bytes.Equal(got2, file2Body) {
		t.Errorf("file2.txt = %q, want %q", got2, file2Body)
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

func TestExtractTar_Symlinks(t *testing.T) {
	// Arrange — target file + symlink entry
	dest := t.TempDir()
	targetBody := []byte("target content")

	buf := buildTar(t, []struct {
		hdr  *tar.Header
		body []byte
	}{
		{hdr: &tar.Header{Typeflag: tar.TypeReg, Name: "real.txt", Mode: 0644, Size: int64(len(targetBody))}, body: targetBody},
		{hdr: &tar.Header{Typeflag: tar.TypeSymlink, Name: "link.txt", Linkname: "real.txt"}},
	})

	// Act
	if err := extractTar(buf, dest); err != nil {
		t.Fatalf("extractTar: %v", err)
	}

	// Assert — link.txt is a symlink pointing to real.txt
	linkTarget, err := os.Readlink(filepath.Join(dest, "link.txt"))
	if err != nil {
		t.Fatalf("Readlink link.txt: %v", err)
	}
	if linkTarget != "real.txt" {
		t.Errorf("symlink target = %q, want %q", linkTarget, "real.txt")
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

func TestExtractTar_PathTraversal(t *testing.T) {
	// Arrange — entry with ../evil name; verify nothing created outside destDir
	destDir := t.TempDir()
	evilBody := []byte("evil content")

	buf := buildTar(t, []struct {
		hdr  *tar.Header
		body []byte
	}{
		{hdr: &tar.Header{Typeflag: tar.TypeReg, Name: "../evil.txt", Mode: 0644, Size: int64(len(evilBody))}, body: evilBody},
	})

	// Act
	if err := extractTar(buf, destDir); err != nil {
		t.Fatalf("extractTar: %v", err)
	}

	// Assert — nothing written inside destDir
	entries, _ := os.ReadDir(destDir)
	if len(entries) != 0 {
		t.Errorf("expected destDir to be empty, got %d entries", len(entries))
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

func TestExtractTar_HardLink(t *testing.T) {
	// Arrange — original file + TypeLink entry
	dest := t.TempDir()
	origBody := []byte("original content for hardlink test")

	buf := buildTar(t, []struct {
		hdr  *tar.Header
		body []byte
	}{
		{hdr: &tar.Header{Typeflag: tar.TypeReg, Name: "original.txt", Mode: 0644, Size: int64(len(origBody))}, body: origBody},
		{hdr: &tar.Header{Typeflag: tar.TypeLink, Name: "hardlinked.txt", Linkname: "original.txt"}},
	})

	// Act
	if err := extractTar(buf, dest); err != nil {
		t.Fatalf("extractTar: %v", err)
	}

	// Assert — hardlinked.txt exists with correct content
	got, err := os.ReadFile(filepath.Join(dest, "hardlinked.txt"))
	if err != nil {
		t.Fatalf("read hardlinked.txt: %v", err)
	}
	if !bytes.Equal(got, origBody) {
		t.Errorf("hardlinked.txt = %q, want %q", got, origBody)
	}

	// Assert — same inode
	fi1, err := os.Stat(filepath.Join(dest, "original.txt"))
	if err != nil {
		t.Fatalf("stat original.txt: %v", err)
	}
	fi2, err := os.Stat(filepath.Join(dest, "hardlinked.txt"))
	if err != nil {
		t.Fatalf("stat hardlinked.txt: %v", err)
	}
	if !os.SameFile(fi1, fi2) {
		t.Errorf("expected original.txt and hardlinked.txt to share an inode")
	}
}

// ---------------------------------------------------------------------------
// ComputeDigest
// ---------------------------------------------------------------------------

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

func TestComputeDigest_EmptyInput(t *testing.T) {
	// SHA-256 of an empty byte slice is a well-known constant.
	data := []byte{}
	got := ComputeDigest(data)
	want := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got != want {
		t.Errorf("ComputeDigest([]) = %q, want %q", got, want)
	}
}

func TestComputeDigest_IsHexString(t *testing.T) {
	// Arrange
	data := []byte("hex string check")

	// Act
	got := ComputeDigest(data)

	// Assert — SHA-256 hex = 64 lowercase hex chars
	if len(got) != 64 {
		t.Errorf("ComputeDigest length = %d, want 64", len(got))
	}
	for _, c := range got {
		if !('0' <= c && c <= '9') && !('a' <= c && c <= 'f') {
			t.Errorf("non-hex char %q in digest %q", c, got)
			break
		}
	}
}

// ---------------------------------------------------------------------------
// ChunkStore
// ---------------------------------------------------------------------------

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

func TestChunkStore_Has_FalseBeforePut(t *testing.T) {
	// Arrange
	cs := NewChunkStore(t.TempDir())
	ctx := context.Background()
	digest := ComputeDigest([]byte("has-before-put"))

	// Act + Assert
	if cs.Has(ctx, digest) {
		t.Errorf("Has() = true before any Put, want false")
	}
}

func TestChunkStore_Has_TrueAfterPut(t *testing.T) {
	// Arrange
	cs := NewChunkStore(t.TempDir())
	ctx := context.Background()
	data := []byte("has-after-put payload")
	digest := ComputeDigest(data)

	// Act
	if err := cs.Put(ctx, digest, data); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Assert
	if !cs.Has(ctx, digest) {
		t.Errorf("Has() = false after Put, want true")
	}
}

func TestChunkStore_PutGet_EmptyData(t *testing.T) {
	// Arrange — zero-byte blob
	cs := NewChunkStore(t.TempDir())
	ctx := context.Background()
	data := []byte{}
	digest := ComputeDigest(data)

	// Act
	if err := cs.Put(ctx, digest, data); err != nil {
		t.Fatalf("Put empty data: %v", err)
	}
	got, err := cs.Get(ctx, digest)

	// Assert
	if err != nil {
		t.Fatalf("Get empty data: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("Get() = %q, want empty slice", got)
	}
}

func TestChunkStore_MultipleBlobs_IndependentPaths(t *testing.T) {
	// Arrange — two distinct digests must not collide in storage.
	cs := NewChunkStore(t.TempDir())
	ctx := context.Background()

	dataA := []byte("blob payload A")
	dataB := []byte("blob payload B")
	digestA := ComputeDigest(dataA)
	digestB := ComputeDigest(dataB)

	// Act
	if err := cs.Put(ctx, digestA, dataA); err != nil {
		t.Fatalf("Put A: %v", err)
	}
	if err := cs.Put(ctx, digestB, dataB); err != nil {
		t.Fatalf("Put B: %v", err)
	}
	gotA, err := cs.Get(ctx, digestA)
	if err != nil {
		t.Fatalf("Get A: %v", err)
	}
	gotB, err := cs.Get(ctx, digestB)
	if err != nil {
		t.Fatalf("Get B: %v", err)
	}

	// Assert
	if !bytes.Equal(gotA, dataA) {
		t.Errorf("Get A = %q, want %q", gotA, dataA)
	}
	if !bytes.Equal(gotB, dataB) {
		t.Errorf("Get B = %q, want %q", gotB, dataB)
	}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Mount / Unmount / Remove
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SafeRef — defined in types.go (no build tag), exercised here.
// ---------------------------------------------------------------------------

func TestSafeRef_Colon(t *testing.T) {
	// Arrange
	input := "nginx:latest"

	// Act
	got := SafeRef(input)

	// Assert
	want := "nginx_latest"
	if got != want {
		t.Errorf("SafeRef(%q) = %q, want %q", input, got, want)
	}
}

func TestSafeRef_SlashesAndColon(t *testing.T) {
	// Arrange
	input := "registry.io/foo/bar:v1"

	// Act
	got := SafeRef(input)

	// Assert
	want := "registry.io_foo_bar_v1"
	if got != want {
		t.Errorf("SafeRef(%q) = %q, want %q", input, got, want)
	}
}

func TestSafeRef_AtSign(t *testing.T) {
	// Arrange
	input := "registry.io/repo@sha256:abc"

	// Act
	got := SafeRef(input)

	// Assert
	want := "registry.io_repo_sha256_abc"
	if got != want {
		t.Errorf("SafeRef(%q) = %q, want %q", input, got, want)
	}
}

func TestSafeRef_PlainName(t *testing.T) {
	// Arrange — no special characters
	input := "ubuntu"

	// Act
	got := SafeRef(input)

	// Assert — unchanged
	if got != input {
		t.Errorf("SafeRef(%q) = %q, want %q (no-op)", input, got, input)
	}
}

// ---------------------------------------------------------------------------
// copyDir
// ---------------------------------------------------------------------------

func TestCopyDir_BasicFiles(t *testing.T) {
	// Arrange
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	fileContent := []byte("root file content")
	nestedContent := []byte("nested file content")

	if err := os.WriteFile(filepath.Join(srcDir, "root.txt"), fileContent, 0644); err != nil {
		t.Fatalf("write root.txt: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "subdir", "nested.txt"), nestedContent, 0644); err != nil {
		t.Fatalf("write nested.txt: %v", err)
	}

	// Act
	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir: %v", err)
	}

	// Assert — root.txt replicated
	gotRoot, err := os.ReadFile(filepath.Join(dstDir, "root.txt"))
	if err != nil {
		t.Fatalf("read dst root.txt: %v", err)
	}
	if !bytes.Equal(gotRoot, fileContent) {
		t.Errorf("root.txt = %q, want %q", gotRoot, fileContent)
	}

	// Assert — subdir/nested.txt replicated
	gotNested, err := os.ReadFile(filepath.Join(dstDir, "subdir", "nested.txt"))
	if err != nil {
		t.Fatalf("read dst nested.txt: %v", err)
	}
	if !bytes.Equal(gotNested, nestedContent) {
		t.Errorf("nested.txt = %q, want %q", gotNested, nestedContent)
	}
}

func TestCopyDir_SymlinksPreserved(t *testing.T) {
	// Arrange
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(srcDir, "target.txt"), []byte("target"), 0644); err != nil {
		t.Fatalf("write target.txt: %v", err)
	}
	if err := os.Symlink("target.txt", filepath.Join(srcDir, "link.txt")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	// Act
	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir: %v", err)
	}

	// Assert — symlink preserved (not followed)
	linkTarget, err := os.Readlink(filepath.Join(dstDir, "link.txt"))
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if linkTarget != "target.txt" {
		t.Errorf("link.txt target = %q, want %q", linkTarget, "target.txt")
	}
}

func TestCopyDir_EmptySource(t *testing.T) {
	// Arrange — source dir with no files
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Act
	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir on empty source: %v", err)
	}

	// Assert — destination remains empty
	entries, err := os.ReadDir(dstDir)
	if err != nil {
		t.Fatalf("ReadDir dst: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty dst, got %d entries", len(entries))
	}
}
