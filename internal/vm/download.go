//go:build !linux

package vm

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	VMImageBaseURL = "https://github.com/thakurprasadrout/thrive/releases/latest"
)

func DownloadVMImage(ctx context.Context) error {
	destDir := filepath.Join(ThriveDir(), "vm")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// Local override: skip network download entirely. Used for air-gapped
	// installs, dev loops, and bypassing missing GitHub release assets.
	if localPath := os.Getenv("THRIVE_VM_IMAGE_PATH"); localPath != "" {
		f, err := os.Open(localPath)
		if err != nil {
			return fmt.Errorf("THRIVE_VM_IMAGE_PATH %q: %w", localPath, err)
		}
		defer f.Close()
		return extractTarGz(f, destDir)
	}

	var artifactName string
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			artifactName = "thrive-vm-darwin-arm64.tar.gz"
		} else {
			artifactName = "thrive-vm-darwin-amd64.tar.gz"
		}
	case "windows":
		artifactName = "thrive-vm-windows-amd64.tar.gz"
	default:
		return fmt.Errorf("unsupported OS for VM download: %s", runtime.GOOS)
	}

	url := VMImageBaseURL + "/" + artifactName

	log.Printf("downloading VM image from %s", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download VM image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	return extractTarGz(resp.Body, destDir)
}

func extractTarGz(r io.Reader, dest string) error {
	gzipRdr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzipRdr.Close()

	tarRdr := tar.NewReader(gzipRdr)
	for {
		header, err := tarRdr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(f, tarRdr); err != nil {
				return err
			}
			if err := os.Chmod(target, header.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeSymlink:
			os.Symlink(header.Linkname, target)
		}
	}
	return nil
}

func VersionFilePath() string {
	return filepath.Join(ThriveDir(), "vm", "downloaded-version.json")
}

func WriteVersionFile(version, tag string) error {
	data := fmt.Sprintf(`{"version": %q, "tag": %q, "downloaded_at": %q}`,
		version, tag, time.Now().Format(time.RFC3339))
	return os.WriteFile(VersionFilePath(), []byte(data), 0644)
}