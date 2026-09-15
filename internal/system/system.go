//go:build linux
// +build linux

// Package system implements disk-usage reporting and pruning of unused
// resources (docker system df/prune parity). Shared by the CLI and the
// thrived daemon.
package system

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/network"
	"github.com/thakurprasadrout/thrive/internal/volume"
)

const (
	imagesDir     = "/var/lib/thrive/images"
	containersDir = "/run/thrive/containers"
	cacheDir      = "/var/lib/thrive/cache"
)

// UsageEntry summarises one resource class for `system df`.
type UsageEntry struct {
	Total       int   `json:"total"`
	Active      int   `json:"active"`
	Size        int64 `json:"size"`
	Reclaimable int64 `json:"reclaimable"`
}

// Usage is the full `system df` report.
type Usage struct {
	Images     UsageEntry `json:"images"`
	Containers UsageEntry `json:"containers"`
	Volumes    UsageEntry `json:"volumes"`
	Networks   UsageEntry `json:"networks"`
	BuildCache UsageEntry `json:"buildCache"`
}

// dirSize returns the total size of regular files under dir.
func dirSize(dir string) int64 {
	var total int64
	filepath.Walk(dir, func(_ string, fi os.FileInfo, err error) error { //nolint:errcheck
		if err == nil && fi.Mode().IsRegular() {
			total += fi.Size()
		}
		return nil
	})
	return total
}

type containerRef struct {
	ID     string
	Image  string
	Status string
}

func scanContainers() []containerRef {
	entries, err := os.ReadDir(containersDir)
	if err != nil {
		return nil
	}
	var out []containerRef
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		var st struct {
			Status string `json:"status"`
		}
		if data, err := os.ReadFile(filepath.Join(containersDir, e.Name(), "state.json")); err == nil {
			json.Unmarshal(data, &st) //nolint:errcheck
		}
		var cfg struct {
			Image string `json:"Image"`
		}
		if data, err := os.ReadFile(filepath.Join(containersDir, e.Name(), "config.json")); err == nil {
			json.Unmarshal(data, &cfg) //nolint:errcheck
		}
		out = append(out, containerRef{ID: e.Name(), Image: cfg.Image, Status: st.Status})
	}
	return out
}

// DiskUsage computes the resource report. Missing stores count as zero.
func DiskUsage() (*Usage, error) {
	ctx := context.Background()
	u := &Usage{}

	imgs, _ := image.List(ctx)
	usedImages := map[string]bool{}
	containers := scanContainers()
	running := 0
	var containerBytes, stoppedBytes int64
	for _, c := range containers {
		sz := dirSize(filepath.Join(containersDir, c.ID))
		containerBytes += sz
		if c.Status == "running" {
			running++
		} else {
			stoppedBytes += sz
		}
		if c.Image != "" {
			usedImages[image.SafeRef(c.Image)] = true
		}
	}
	u.Containers = UsageEntry{Total: len(containers), Active: running, Size: containerBytes, Reclaimable: stoppedBytes}

	var imageBytes, unusedBytes int64
	active := 0
	for _, img := range imgs {
		sz := dirSize(filepath.Join(imagesDir, image.SafeRef(img.Ref)))
		imageBytes += sz
		if usedImages[image.SafeRef(img.Ref)] {
			active++
		} else {
			unusedBytes += sz
		}
	}
	u.Images = UsageEntry{Total: len(imgs), Active: active, Size: imageBytes, Reclaimable: unusedBytes}

	vols, _ := volume.List()
	var volBytes int64
	for _, v := range vols {
		volBytes += dirSize(v.Path)
	}
	inUse := volumeInUse()
	activeVols := 0
	for _, v := range vols {
		if inUse[v.Path] {
			activeVols++
		}
	}
	u.Volumes = UsageEntry{Total: len(vols), Active: activeVols, Size: volBytes, Reclaimable: volBytes - dirSizeOfUsed(vols, inUse)}

	nets, _ := network.ListNetworks()
	activeNets := 0
	for _, nw := range nets {
		if len(nw.Containers) > 0 {
			activeNets++
		}
	}
	u.Networks = UsageEntry{Total: len(nets), Active: activeNets}

	u.BuildCache = UsageEntry{Size: dirSize(cacheDir), Reclaimable: dirSize(cacheDir)}
	return u, nil
}

func dirSizeOfUsed(vols []*volume.Volume, inUse map[string]bool) int64 {
	var total int64
	for _, v := range vols {
		if inUse[v.Path] {
			total += dirSize(v.Path)
		}
	}
	return total
}

func volumeInUse() map[string]bool {
	inUse := map[string]bool{}
	for _, c := range scanContainers() {
		data, err := os.ReadFile(filepath.Join(containersDir, c.ID, "config.json"))
		if err != nil {
			continue
		}
		var cfg struct {
			Mounts []struct {
				Source string `json:"Source"`
			} `json:"Mounts"`
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}
		for _, m := range cfg.Mounts {
			inUse[m.Source] = true
		}
	}
	return inUse
}

// PruneOptions controls `system prune`.
type PruneOptions struct {
	AllImages bool // remove all unreferenced images (default: none)
	Volumes   bool // also prune unused volumes
}

// PruneReport summarises a prune run.
type PruneReport struct {
	ContainersDeleted int   `json:"containersDeleted"`
	ImagesDeleted     int   `json:"imagesDeleted"`
	VolumesDeleted    int   `json:"volumesDeleted"`
	NetworksDeleted   int   `json:"networksDeleted"`
	SpaceReclaimed    int64 `json:"spaceReclaimed"`
}

// Prune removes stopped containers, unused networks, and optionally
// unreferenced images and unused volumes.
func Prune(opts PruneOptions) (*PruneReport, error) {
	rep := &PruneReport{}
	containers := scanContainers()

	// Stopped containers.
	for _, c := range containers {
		if c.Status == "running" || c.Status == "paused" || c.Status == "created" {
			continue
		}
		rep.SpaceReclaimed += dirSize(filepath.Join(containersDir, c.ID))
		network.DetachContainer(c.ID)
		if err := os.RemoveAll(filepath.Join(containersDir, c.ID)); err == nil {
			rep.ContainersDeleted++
		}
	}

	// Unused networks.
	if removed, err := network.PruneNetworks(); err == nil {
		rep.NetworksDeleted = len(removed)
	}

	// Images: only with --all (unreferenced by surviving containers).
	if opts.AllImages {
		ctx := context.Background()
		used := map[string]bool{}
		for _, c := range scanContainers() {
			if c.Image != "" {
				used[image.SafeRef(c.Image)] = true
			}
		}
		if imgs, err := image.List(ctx); err == nil {
			for _, img := range imgs {
				safe := image.SafeRef(img.Ref)
				if used[safe] {
					continue
				}
				rep.SpaceReclaimed += dirSize(filepath.Join(imagesDir, safe))
				// image.Remove addresses the raw ref path; the store uses
				// SafeRef, so remove the store directory directly.
				if err := os.RemoveAll(filepath.Join(imagesDir, safe)); err == nil {
					rep.ImagesDeleted++
				}
			}
		}
	}

	// Volumes with --volumes.
	if opts.Volumes {
		inUse := volumeInUse()
		if vols, err := volume.List(); err == nil {
			for _, v := range vols {
				if !inUse[v.Path] {
					rep.SpaceReclaimed += dirSize(v.Path)
				}
			}
		}
		if removed, err := volume.Prune(func(path string) bool {
			return inUse[path]
		}); err == nil {
			rep.VolumesDeleted = len(removed)
		}
	}

	return rep, nil
}
