package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/thakurprasadrout/thrive/internal/image"
)

// ManifestInfo summarises a remote manifest for `thrive manifest inspect`.
type ManifestInfo struct {
	Ref       string          `json:"ref"`
	Digest    string          `json:"digest"`
	MediaType string          `json:"mediaType"`
	Layers    []ManifestLayer `json:"layers,omitempty"`
	Manifests []string        `json:"manifests,omitempty"`
}

// ManifestLayer describes one layer in a remote image manifest.
type ManifestLayer struct {
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

// ManifestInspect fetches the remote manifest for ref.
func ManifestInspect(ctx context.Context, ref string, authUser, authPass string) (*ManifestInfo, error) {
	parsed, err := name.ParseReference(ref)
	if err != nil {
		return nil, fmt.Errorf("registry: manifest: parse %q: %w", ref, err)
	}
	desc, err := remote.Get(parsed,
		remote.WithAuth(ResolveAuth(ref, authUser, authPass)),
		remote.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("registry: manifest: fetch %q: %w", ref, err)
	}
	info := &ManifestInfo{Ref: parsed.String(), Digest: desc.Digest.String(), MediaType: string(desc.MediaType)}
	if desc.MediaType.IsIndex() {
		idx, err := desc.ImageIndex()
		if err != nil {
			return nil, fmt.Errorf("registry: manifest: index: %w", err)
		}
		manifest, err := idx.IndexManifest()
		if err != nil {
			return nil, fmt.Errorf("registry: manifest: read index: %w", err)
		}
		for _, m := range manifest.Manifests {
			platform := ""
			if m.Platform != nil {
				platform = m.Platform.OS + "/" + m.Platform.Architecture
			}
			info.Manifests = append(info.Manifests, m.Digest.String()+" "+platform)
		}
		return info, nil
	}
	img, err := desc.Image()
	if err != nil {
		return nil, fmt.Errorf("registry: manifest: image: %w", err)
	}
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("registry: manifest: layers: %w", err)
	}
	for _, l := range layers {
		digest, err := l.Digest()
		if err != nil {
			continue
		}
		size, _ := l.Size()
		info.Layers = append(info.Layers, ManifestLayer{Digest: digest.String(), Size: size})
	}
	return info, nil
}

// ManifestEntry is one image in a local manifest list.
type ManifestEntry struct {
	Ref          string            `json:"ref"`
	OS           string            `json:"os,omitempty"`
	Architecture string            `json:"architecture,omitempty"`
	Variant      string            `json:"variant,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
}

// ManifestList is a locally stored multi-arch manifest list.
type ManifestList struct {
	Name    string          `json:"name"`
	Entries []ManifestEntry `json:"entries"`
}

func manifestStoreDir() string {
	if manifestStoreOverride != "" {
		return manifestStoreOverride
	}
	base := image.StoreDir()
	if base == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(base), "manifests")
}

func manifestPath(listName string) string {
	return filepath.Join(manifestStoreDir(), image.SafeRef(listName)+".json")
}

func readManifestList(listName string) (*ManifestList, error) {
	data, err := os.ReadFile(manifestPath(listName))
	if err != nil {
		return nil, fmt.Errorf("registry: manifest list %q not found: %w", listName, err)
	}
	var list ManifestList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("registry: manifest list: parse: %w", err)
	}
	return &list, nil
}

func writeManifestList(list *ManifestList) error {
	if manifestStoreDir() == "" {
		return fmt.Errorf("registry: manifest store unavailable on this platform")
	}
	if err := os.MkdirAll(manifestStoreDir(), 0755); err != nil {
		return fmt.Errorf("registry: manifest: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("registry: manifest: marshal: %w", err)
	}
	if err := os.WriteFile(manifestPath(list.Name), data, 0644); err != nil {
		return fmt.Errorf("registry: manifest: write: %w", err)
	}
	return nil
}

// ManifestCreate stores a new local manifest list.
func ManifestCreate(listName string, refs []string) error {
	if listName == "" || len(refs) == 0 {
		return fmt.Errorf("registry: manifest create requires a name and at least one image")
	}
	list := &ManifestList{Name: listName}
	for _, r := range refs {
		list.Entries = append(list.Entries, ManifestEntry{Ref: r})
	}
	return writeManifestList(list)
}

// ManifestRemove deletes a local manifest list.
func ManifestRemove(listName string) error {
	if err := os.Remove(manifestPath(listName)); err != nil {
		return fmt.Errorf("registry: manifest rm: %w", err)
	}
	return nil
}

// ManifestAnnotate sets platform metadata on one entry of a local list.
func ManifestAnnotate(listName, ref, osName, arch, variant string, annotations map[string]string) error {
	list, err := readManifestList(listName)
	if err != nil {
		return err
	}
	found := false
	for i, e := range list.Entries {
		if e.Ref == ref {
			list.Entries[i].OS = osName
			list.Entries[i].Architecture = arch
			list.Entries[i].Variant = variant
			if annotations != nil {
				list.Entries[i].Annotations = annotations
			}
			found = true
		}
	}
	if !found {
		return fmt.Errorf("registry: manifest annotate: image %q not in list %q", ref, listName)
	}
	return writeManifestList(list)
}

// ManifestPush assembles the local list into a remote OCI index.
func ManifestPush(ctx context.Context, listName, authUser, authPass string) error {
	list, err := readManifestList(listName)
	if err != nil {
		return err
	}
	dest, err := name.ParseReference(listName)
	if err != nil {
		return fmt.Errorf("registry: manifest push: parse %q: %w", listName, err)
	}
	return pushManifestList(ctx, dest, list, authUser, authPass)
}

func pushManifestList(ctx context.Context, dest name.Reference, list *ManifestList, authUser, authPass string) error {
	var adds []mutate.IndexAddendum
	for _, e := range list.Entries {
		src, err := name.ParseReference(e.Ref)
		if err != nil {
			return fmt.Errorf("registry: manifest push: parse %q: %w", e.Ref, err)
		}
		desc, err := remote.Get(src,
			remote.WithAuth(ResolveAuth(e.Ref, authUser, authPass)),
			remote.WithContext(ctx),
		)
		if err != nil {
			return fmt.Errorf("registry: manifest push: fetch %q: %w", e.Ref, err)
		}
		add := mutate.IndexAddendum{}
		if desc.MediaType.IsIndex() {
			idx, err := desc.ImageIndex()
			if err != nil {
				return fmt.Errorf("registry: manifest push: index %q: %w", e.Ref, err)
			}
			add.Add = idx
		} else {
			img, err := desc.Image()
			if err != nil {
				return fmt.Errorf("registry: manifest push: image %q: %w", e.Ref, err)
			}
			add.Add = img
		}
		if e.OS != "" || e.Architecture != "" {
			add.Platform = &v1.Platform{OS: e.OS, Architecture: e.Architecture, Variant: e.Variant}
		}
		if e.Annotations != nil {
			add.Annotations = e.Annotations
		}
		adds = append(adds, add)
	}
	idx := mutate.AppendManifests(
		mutate.IndexMediaType(empty.Index, types.MediaType("application/vnd.oci.image.index.v1+json")),
		adds...,
	)
	if err := remote.WriteIndex(dest, idx, remote.WithAuth(ResolveAuth(dest.String(), authUser, authPass))); err != nil {
		return fmt.Errorf("registry: manifest push: write: %w", err)
	}
	return nil
}
