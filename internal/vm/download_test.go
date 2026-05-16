//go:build !linux

package vm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// makeTarGz writes a single-file tar.gz at dir/name and returns the path.
func makeTarGz(t *testing.T, dir, name, fileName string, body []byte) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{
		Name:     fileName,
		Mode:     0644,
		Size:     int64(len(body)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gz.Close()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDownloadVMImage_UsesLocalPathFromEnv(t *testing.T) {
	tmp := t.TempDir()
	tarball := makeTarGz(t, tmp, "thrive-vm.tar.gz", "kernel", []byte("KERNEL_BYTES"))

	thriveHome := t.TempDir()
	t.Setenv("HOME", thriveHome)
	t.Setenv("LOCALAPPDATA", thriveHome)
	t.Setenv("THRIVE_VM_IMAGE_PATH", tarball)

	if err := DownloadVMImage(context.Background()); err != nil {
		t.Fatalf("DownloadVMImage with local override failed: %v", err)
	}

	extracted := filepath.Join(ThriveDir(), "vm", "kernel")
	got, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatalf("expected extracted kernel at %s: %v", extracted, err)
	}
	if string(got) != "KERNEL_BYTES" {
		t.Errorf("kernel contents mismatch: got %q", string(got))
	}
}

func TestDownloadVMImage_LocalPathMissingReturnsError(t *testing.T) {
	t.Setenv("THRIVE_VM_IMAGE_PATH", "/nonexistent/path/that/should/not/exist.tar.gz")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())

	if err := DownloadVMImage(context.Background()); err == nil {
		t.Fatal("expected error for missing local image path")
	}
}
