//go:build linux
// +build linux

package runtime

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/thakurprasadrout/thrive/internal/cgroup"
	"github.com/thakurprasadrout/thrive/internal/image"
)

// ContainerStats is a point-in-time resource snapshot for `thrive stats`.
type ContainerStats struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	PID           int    `json:"pid"`
	MemoryCurrent int64  `json:"memoryCurrent"`
	MemoryLimit   int64  `json:"memoryLimit"`
	CPUUsageUsec  int64  `json:"cpuUsageUsec"`
	PIDsCurrent   int64  `json:"pidsCurrent"`
	Frozen        bool   `json:"frozen"`
}

// ProcessInfo describes one host-visible process for `thrive top`.
type ProcessInfo struct {
	PID  int    `json:"pid"`
	PPID int    `json:"ppid"`
	Cmd  string `json:"cmd"`
}

// FileChange describes one changed file for `thrive diff`.
type FileChange struct {
	Kind string `json:"kind"` // A (added), C (changed), D (deleted)
	Path string `json:"path"`
}

// UpdateOptions holds live-updateable resource limits for `thrive update`.
type UpdateOptions struct {
	MemoryLimit int64
	CPUQuota    int64
	CPUShares   int64
	PIDsLimit   int64
}

// Pause freezes a running container via the cgroup v2 freezer.
func Pause(ctx context.Context, id string) error {
	state, err := loadState(id)
	if err != nil {
		return fmt.Errorf("runtime.Pause: %w", err)
	}
	if state.Status != "running" {
		return fmt.Errorf("runtime.Pause: container %s is %s (not running)", id, state.Status)
	}
	mgr, err := cgroup.New(id)
	if err != nil {
		return fmt.Errorf("runtime.Pause: cgroup: %w", err)
	}
	if err := mgr.Freeze(); err != nil {
		return fmt.Errorf("runtime.Pause: freeze: %w", err)
	}
	state.Status = "paused"
	if err := saveState(containerDir(id), state); err != nil {
		return fmt.Errorf("runtime.Pause: saveState: %w", err)
	}
	return nil
}

// Unpause resumes a paused container.
func Unpause(ctx context.Context, id string) error {
	state, err := loadState(id)
	if err != nil {
		return fmt.Errorf("runtime.Unpause: %w", err)
	}
	if state.Status != "paused" {
		return fmt.Errorf("runtime.Unpause: container %s is %s (not paused)", id, state.Status)
	}
	mgr, err := cgroup.New(id)
	if err != nil {
		return fmt.Errorf("runtime.Unpause: cgroup: %w", err)
	}
	if err := mgr.Unfreeze(); err != nil {
		return fmt.Errorf("runtime.Unpause: unfreeze: %w", err)
	}
	state.Status = "running"
	if err := saveState(containerDir(id), state); err != nil {
		return fmt.Errorf("runtime.Unpause: saveState: %w", err)
	}
	return nil
}

// Wait blocks until the container stops and returns its exit code.
func Wait(ctx context.Context, id string) (int, error) {
	if _, err := loadState(id); err != nil {
		return 0, fmt.Errorf("runtime.Wait: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("runtime.Wait: %w", ctx.Err())
		default:
		}
		state, err := loadState(id)
		if err != nil {
			return 0, fmt.Errorf("runtime.Wait: %w", err)
		}
		if state.Status == "stopped" {
			return state.ExitCode, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// Rename renames a container by moving its state directory.
func Rename(ctx context.Context, oldID, newID string) error {
	if oldID == "" || newID == "" {
		return fmt.Errorf("runtime.Rename: container ID required")
	}
	if _, err := loadState(oldID); err != nil {
		return fmt.Errorf("runtime.Rename: %w", err)
	}
	if _, err := loadState(newID); err == nil {
		return fmt.Errorf("runtime.Rename: container %s already exists", newID)
	}
	oldDir := containerDir(oldID)
	newDir := containerDir(newID)
	if err := os.Rename(oldDir, newDir); err != nil {
		return fmt.Errorf("runtime.Rename: rename: %w", err)
	}
	// Update ID fields inside config.json and state.json (best-effort).
	if data, err := os.ReadFile(filepath.Join(newDir, "config.json")); err == nil {
		var cfg ContainerConfig
		if json.Unmarshal(data, &cfg) == nil {
			cfg.ID = newID
			if updated, err := json.Marshal(cfg); err == nil {
				os.WriteFile(filepath.Join(newDir, "config.json"), updated, 0644) //nolint:errcheck
			}
		}
	}
	if st, err := loadState(newID); err == nil {
		st.ID = newID
		saveState(newDir, st) //nolint:errcheck
	}
	return nil
}

// Stats returns resource usage for a container.
func Stats(ctx context.Context, id string) (*ContainerStats, error) {
	state, err := loadState(id)
	if err != nil {
		return nil, fmt.Errorf("runtime.Stats: %w", err)
	}
	cg, _ := cgroup.ReadStats(id)
	var memLimit int64
	if data, err := os.ReadFile(filepath.Join(containerDir(id), "config.json")); err == nil {
		var cfg ContainerConfig
		if json.Unmarshal(data, &cfg) == nil {
			memLimit = cfg.Resources.MemoryLimit
		}
	}
	return &ContainerStats{
		ID:            state.ID,
		Status:        state.Status,
		PID:           state.PID,
		MemoryCurrent: cg.MemoryCurrent,
		MemoryLimit:   memLimit,
		CPUUsageUsec:  cg.CPUUsageUsec,
		PIDsCurrent:   cg.PIDsCurrent,
		Frozen:        cg.Frozen,
	}, nil
}

// Update applies new resource limits live and persists them to config.json.
func Update(ctx context.Context, id string, opts UpdateOptions) error {
	if _, err := loadState(id); err != nil {
		return fmt.Errorf("runtime.Update: %w", err)
	}
	mgr, err := cgroup.New(id)
	if err != nil {
		return fmt.Errorf("runtime.Update: cgroup: %w", err)
	}
	if opts.MemoryLimit > 0 {
		if err := mgr.SetMemoryLimit(opts.MemoryLimit); err != nil {
			return fmt.Errorf("runtime.Update: memory: %w", err)
		}
	}
	if opts.CPUQuota > 0 {
		if err := mgr.SetCPUQuota(opts.CPUQuota); err != nil {
			return fmt.Errorf("runtime.Update: cpu: %w", err)
		}
	}
	if opts.CPUShares > 0 {
		if err := mgr.SetCPUShares(opts.CPUShares); err != nil {
			return fmt.Errorf("runtime.Update: cpu-shares: %w", err)
		}
	}
	if opts.PIDsLimit > 0 {
		if err := mgr.SetPIDsLimit(opts.PIDsLimit); err != nil {
			return fmt.Errorf("runtime.Update: pids: %w", err)
		}
	}
	// Persist to config.json.
	cfgPath := filepath.Join(containerDir(id), "config.json")
	if data, err := os.ReadFile(cfgPath); err == nil {
		var cfg ContainerConfig
		if json.Unmarshal(data, &cfg) == nil {
			if opts.MemoryLimit > 0 {
				cfg.Resources.MemoryLimit = opts.MemoryLimit
			}
			if opts.CPUQuota > 0 {
				cfg.Resources.CPUQuota = opts.CPUQuota
			}
			if opts.CPUShares > 0 {
				cfg.Resources.CPUShares = opts.CPUShares
			}
			if updated, err := json.Marshal(cfg); err == nil {
				os.WriteFile(cfgPath, updated, 0644) //nolint:errcheck
			}
		}
	}
	return nil
}

// Top lists host-visible processes belonging to the container.
func Top(ctx context.Context, id string) ([]ProcessInfo, error) {
	state, err := loadState(id)
	if err != nil {
		return nil, fmt.Errorf("runtime.Top: %w", err)
	}
	if state.PID <= 0 {
		return nil, fmt.Errorf("runtime.Top: container %s is not running", id)
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("runtime.Top: read /proc: %w", err)
	}
	var out []ProcessInfo
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		statData, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat"))
		if err != nil {
			continue
		}
		// comm may contain spaces/parens; ppid is the 4th field after comm.
		s := string(statData)
		closeIdx := strings.LastIndex(s, ")")
		if closeIdx < 0 {
			continue
		}
		fields := strings.Fields(s[closeIdx+1:])
		if len(fields) < 2 {
			continue
		}
		ppid, _ := strconv.Atoi(fields[1])
		if pid != state.PID && ppid != state.PID {
			continue
		}
		cmd := ""
		if cmdline, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline")); err == nil {
			cmd = strings.ReplaceAll(strings.TrimRight(string(cmdline), "\x00"), "\x00", " ")
			if cmd == "" {
				cmd = "[" + e.Name() + "]"
			}
		}
		out = append(out, ProcessInfo{PID: pid, PPID: ppid, Cmd: cmd})
	}
	return out, nil
}

// ContainerPorts returns the port mappings from the container config.
func ContainerPorts(ctx context.Context, id string) ([]PortMapping, error) {
	data, err := os.ReadFile(filepath.Join(containerDir(id), "config.json"))
	if err != nil {
		return nil, fmt.Errorf("runtime.Port: read config: %w", err)
	}
	var cfg ContainerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("runtime.Port: unmarshal config: %w", err)
	}
	return cfg.Ports, nil
}

// Diff lists files changed in the container's writable layer.
func Diff(ctx context.Context, id string) ([]FileChange, error) {
	upperDir := filepath.Join(containerDir(id), "upper")
	if _, err := os.Stat(upperDir); err != nil {
		return nil, nil
	}
	var out []FileChange
	err := filepath.Walk(upperDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(upperDir, path)
		if rel == "." {
			return nil
		}
		out = append(out, FileChange{Kind: "A", Path: "/" + rel})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("runtime.Diff: walk: %w", err)
	}
	return out, nil
}

// Export tars the container's filesystem to w.
func Export(ctx context.Context, id string, w io.Writer) error {
	if _, err := loadState(id); err != nil {
		return fmt.Errorf("runtime.Export: %w", err)
	}
	srcDir := filepath.Join(containerDir(id), "merged")
	if _, err := os.Stat(srcDir); err != nil {
		srcDir = filepath.Join(containerDir(id), "upper")
	}
	if _, err := os.Stat(srcDir); err != nil {
		return fmt.Errorf("runtime.Export: no filesystem for container %s", id)
	}
	if err := tarDir(srcDir, w); err != nil {
		return fmt.Errorf("runtime.Export: %w", err)
	}
	return nil
}

// Commit snapshots the container's writable layer as a new local image.
func Commit(ctx context.Context, id, newRef string) error {
	if _, err := loadState(id); err != nil {
		return fmt.Errorf("runtime.Commit: %w", err)
	}
	if newRef == "" {
		return fmt.Errorf("runtime.Commit: image reference required")
	}
	upperDir := filepath.Join(containerDir(id), "upper")
	if _, err := os.Stat(upperDir); err != nil {
		return fmt.Errorf("runtime.Commit: no writable layer for container %s", id)
	}
	sum := sha256.Sum256([]byte(newRef + id + time.Now().UTC().String()))
	digest := "sha256:" + hex.EncodeToString(sum[:])
	imgDir := filepath.Join("/var/lib/thrive/images", image.SafeRef(newRef))
	layerDir := filepath.Join(imgDir, "layers", digest)
	if err := os.MkdirAll(layerDir, 0755); err != nil {
		return fmt.Errorf("runtime.Commit: mkdir: %w", err)
	}
	if err := copyDir(upperDir, layerDir); err != nil {
		return fmt.Errorf("runtime.Commit: copy: %w", err)
	}
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
		return fmt.Errorf("runtime.Commit: marshal: %w", err)
	}
	if err := os.WriteFile(filepath.Join(imgDir, "manifest.json"), metaJSON, 0644); err != nil {
		return fmt.Errorf("runtime.Commit: write manifest: %w", err)
	}
	return nil
}

func containerDir(id string) string {
	return filepath.Join("/run/thrive/containers", id)
}

func tarDir(srcDir string, w io.Writer) error {
	tw := tar.NewWriter(w)
	defer tw.Close()
	return filepath.Walk(srcDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(srcDir, path)
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
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, fi.Mode())
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			os.Remove(target)
			return os.Symlink(link, target)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fi.Mode())
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}
