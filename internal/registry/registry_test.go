package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/thakurprasadrout/thrive/internal/image"
)

func testAuthOverride(t *testing.T) {
	t.Helper()
	authFileOverride = filepath.Join(t.TempDir(), "auth.json")
	manifestStoreOverride = t.TempDir()
	t.Cleanup(func() {
		authFileOverride = ""
		manifestStoreOverride = ""
	})
}

// TestRegistryHost_DockerHub verifies docker.io normalisation.
func TestRegistryHost_DockerHub(t *testing.T) {
	if got := RegistryHost("alpine"); got != "https://index.docker.io/v1/" {
		t.Errorf("RegistryHost(alpine): got %q", got)
	}
	if got := RegistryHost("library/alpine:latest"); got != "https://index.docker.io/v1/" {
		t.Errorf("RegistryHost(library/alpine): got %q", got)
	}
}

// TestRegistryHost_Custom verifies custom registries keep their hostname.
func TestRegistryHost_Custom(t *testing.T) {
	if got := RegistryHost("ghcr.io/owner/img:v1"); got != "ghcr.io" {
		t.Errorf("RegistryHost(ghcr.io/...): got %q", got)
	}
}

// TestLoginLogout_Roundtrip verifies credential store write/read/delete.
func TestLoginLogout_Roundtrip(t *testing.T) {
	testAuthOverride(t)

	if err := Login("https://index.docker.io/v1/", "alice", "s3cret"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	c := StoredAuth("https://index.docker.io/v1/")
	if c == nil || c.Username != "alice" || c.Password != "s3cret" {
		t.Fatalf("StoredAuth: got %+v", c)
	}
	// Ref-based lookup resolves to the same entry.
	c = StoredAuth(RegistryHost("alpine"))
	if c == nil || c.Username != "alice" {
		t.Fatalf("StoredAuth(ref): got %+v", c)
	}
	if err := Logout("alpine"); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if c := StoredAuth("https://index.docker.io/v1/"); c != nil {
		t.Errorf("StoredAuth after logout: got %+v, want nil", c)
	}
}

// TestLogout_Missing verifies Logout errors when not logged in.
func TestLogout_Missing(t *testing.T) {
	testAuthOverride(t)
	if err := Logout("ghcr.io"); err == nil {
		t.Error("Logout: expected error, got nil")
	}
}

// TestResolveAuth_Precedence verifies flags beat the store beats anonymous.
func TestResolveAuth_Precedence(t *testing.T) {
	testAuthOverride(t)
	if err := Login("ghcr.io", "bob", "pw"); err != nil {
		t.Fatalf("Login: %v", err)
	}

	if a := ResolveAuth("ghcr.io/o/i:v", "flag-user", "flag-pw"); a == authn.Anonymous {
		t.Error("ResolveAuth: flag creds should not be anonymous")
	}
	if a := ResolveAuth("ghcr.io/o/i:v", "", ""); a == authn.Anonymous {
		t.Error("ResolveAuth: stored creds should not be anonymous")
	}
	if a := ResolveAuth("quay.io/o/i:v", "", ""); a != authn.Anonymous {
		t.Error("ResolveAuth: unknown registry should be anonymous")
	}
}

// fixtureStore builds a minimal image store with one image and one layer file.
func fixtureStore(t *testing.T, ref string) string {
	t.Helper()
	store := t.TempDir()
	layerDir := filepath.Join(store, image.SafeRef(ref), "layers", "sha256:abc")
	if err := os.MkdirAll(layerDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(layerDir, "hello.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	meta := struct {
		Ref    string
		Digest string
		Layers []image.Layer
	}{
		Ref: ref, Digest: "sha256:img",
		Layers: []image.Layer{{Digest: "sha256:abc", Size: 5, Path: layerDir}},
	}
	metaJSON, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(store, image.SafeRef(ref), "manifest.json"), metaJSON, 0644); err != nil {
		t.Fatal(err)
	}
	return store
}

// TestSaveLoad_Roundtrip verifies save output reloads the same image.
func TestSaveLoad_Roundtrip(t *testing.T) {
	ctx := context.Background()
	src := fixtureStore(t, "example.com/app:v1")

	var buf bytes.Buffer
	if err := SaveTo(ctx, src, []string{"example.com/app:v1"}, &buf); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("SaveTo: empty archive")
	}

	dst := t.TempDir()
	loaded, err := LoadFrom(ctx, dst, &buf)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if len(loaded) != 1 || loaded[0] != "example.com/app:v1" {
		t.Fatalf("LoadFrom: got %v", loaded)
	}
	data, err := os.ReadFile(filepath.Join(dst, image.SafeRef("example.com/app:v1"), "layers", "sha256:abc", "hello.txt"))
	if err != nil {
		t.Fatalf("LoadFrom: layer file: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("LoadFrom: layer content %q, want %q", data, "hello")
	}
}

// TestSave_MissingImage verifies Save errors for unknown images.
func TestSave_MissingImage(t *testing.T) {
	var buf bytes.Buffer
	if err := SaveTo(context.Background(), t.TempDir(), []string{"nope:latest"}, &buf); err == nil {
		t.Error("SaveTo: expected error, got nil")
	}
}

// TestLoad_BadArchive verifies Load rejects non-save archives.
func TestLoad_BadArchive(t *testing.T) {
	_, err := LoadFrom(context.Background(), t.TempDir(), bytes.NewReader([]byte("not a tar")))
	if err == nil {
		t.Error("LoadFrom: expected error, got nil")
	}
}

// TestHistoryTo verifies layer listing from a fixture store.
func TestHistoryTo(t *testing.T) {
	store := fixtureStore(t, "example.com/app:v1")
	layers, err := HistoryTo(context.Background(), store, "example.com/app:v1")
	if err != nil {
		t.Fatalf("HistoryTo: %v", err)
	}
	if len(layers) != 1 || layers[0].Digest != "sha256:abc" {
		t.Errorf("HistoryTo: got %+v", layers)
	}
	if _, err := HistoryTo(context.Background(), store, "missing:v1"); err == nil {
		t.Error("HistoryTo: expected error for missing image, got nil")
	}
}

// TestImportTo verifies a tarball becomes a loadable image.
func TestImportTo(t *testing.T) {
	ctx := context.Background()
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "app.bin"), []byte("binary"), 0755); err != nil {
		t.Fatal(err)
	}
	var tarBuf bytes.Buffer
	if err := tarDirToWriter(src, &tarBuf); err != nil {
		t.Fatalf("tar fixture: %v", err)
	}
	store := t.TempDir()
	if err := ImportTo(ctx, store, &tarBuf, "imported:v1"); err != nil {
		t.Fatalf("ImportTo: %v", err)
	}
	layers, err := HistoryTo(ctx, store, "imported:v1")
	if err != nil {
		t.Fatalf("HistoryTo after import: %v", err)
	}
	if len(layers) != 1 {
		t.Fatalf("HistoryTo: got %d layers, want 1", len(layers))
	}
	if err := ImportTo(ctx, store, &tarBuf, ""); err == nil {
		t.Error("ImportTo: expected error for empty ref, got nil")
	}
}

// TestManifestList_CRUD verifies local manifest list lifecycle.
func TestManifestList_CRUD(t *testing.T) {
	testAuthOverride(t)

	if err := ManifestCreate("example.com/multi:latest", []string{"example.com/a:v1", "example.com/b:v1"}); err != nil {
		t.Fatalf("ManifestCreate: %v", err)
	}
	if err := ManifestAnnotate("example.com/multi:latest", "example.com/a:v1", "linux", "amd64", "", nil); err != nil {
		t.Fatalf("ManifestAnnotate: %v", err)
	}
	list, err := readManifestList("example.com/multi:latest")
	if err != nil {
		t.Fatalf("readManifestList: %v", err)
	}
	if len(list.Entries) != 2 || list.Entries[0].OS != "linux" || list.Entries[0].Architecture != "amd64" {
		t.Errorf("manifest list: got %+v", list.Entries)
	}
	if err := ManifestAnnotate("example.com/multi:latest", "missing:v1", "linux", "arm64", "", nil); err == nil {
		t.Error("ManifestAnnotate: expected error for unknown entry, got nil")
	}
	if err := ManifestRemove("example.com/multi:latest"); err != nil {
		t.Fatalf("ManifestRemove: %v", err)
	}
	if err := ManifestRemove("example.com/multi:latest"); err == nil {
		t.Error("ManifestRemove: expected error for missing list, got nil")
	}
}
