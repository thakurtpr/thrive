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
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/thakurprasadrout/thrive/internal/cgroup"
	"github.com/thakurprasadrout/thrive/internal/events"
	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/telemetry"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
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
//
// OverlayFS semantics (docker diff parity):
//   - whiteout char devices (rdev 0/0) and ".wh." prefixed entries mark
//     deletions from a lower layer → reported as D.
//   - opaque directories (xattr trusted.overlay.opaque=y) hide all lower
//     content → the directory itself is reported as C.
//   - any other entry whose relative path also exists in a lower image
//     layer → C (changed); otherwise → A (added).
func Diff(ctx context.Context, id string) ([]FileChange, error) {
	lowerDirs := lowerLayerDirs(id)
	if mountMode(id) == image.MountModeCopy {
		mergedDir := filepath.Join(containerDir(id), "merged")
		if _, err := os.Stat(mergedDir); err != nil {
			return nil, nil
		}
		return diffCopyChanges(mergedDir, lowerDirs)
	}
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
		base := filepath.Base(rel)
		dir := filepath.Dir(rel)
		// Deletion markers: OverlayFS whiteout char 0/0, or .wh. files
		// (aufs-style and tar-export style markers some tools leave behind).
		if strings.HasPrefix(base, ".wh.") {
			out = append(out, FileChange{Kind: "D", Path: "/" + filepath.Join(dir, strings.TrimPrefix(base, ".wh."))})
			return nil
		}
		if fi.Mode()&os.ModeCharDevice != 0 {
			if st, ok := fi.Sys().(*syscall.Stat_t); ok && st.Rdev == 0 {
				out = append(out, FileChange{Kind: "D", Path: "/" + rel})
				return nil
			}
		}
		// Opaque directory: lower content hidden → changed.
		if fi.IsDir() && isOpaqueDir(path) {
			out = append(out, FileChange{Kind: "C", Path: "/" + rel})
			return nil
		}
		if lowerExists(lowerDirs, rel) {
			out = append(out, FileChange{Kind: "C", Path: "/" + rel})
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

// mountMode returns the recorded mount mode, defaulting to overlay for
// containers created before the marker existed.
func mountMode(id string) string {
	data, err := os.ReadFile(filepath.Join(containerDir(id), image.MountModeFile))
	if err != nil {
		return image.MountModeOverlay
	}
	if strings.TrimSpace(string(data)) == image.MountModeCopy {
		return image.MountModeCopy
	}
	return image.MountModeOverlay
}

// diffCopyChanges compares a copy-fallback merged rootfs against the lower
// image layers: entries absent from all lowers → Added; differing content
// → Changed; lower entries absent from merged → Deleted. Directory entries
// are reported Added only when absent from all lowers (present dirs are
// structural, not changes). Output is sorted by path for stability.
func diffCopyChanges(mergedDir string, lowerDirs []string) ([]FileChange, error) {
	var out []FileChange
	seen := map[string]bool{}
	err := filepath.Walk(mergedDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(mergedDir, path)
		if rel == "." {
			return nil
		}
		seen[rel] = true
		lowerPath := lowerFile(lowerDirs, rel)
		if lowerPath == "" {
			out = append(out, FileChange{Kind: "A", Path: "/" + rel})
			return nil
		}
		if fi.IsDir() {
			return nil
		}
		if fileContentDiffers(path, lowerPath) {
			out = append(out, FileChange{Kind: "C", Path: "/" + rel})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("runtime.Diff: walk merged: %w", err)
	}
	reported := map[string]bool{}
	for _, ld := range lowerDirs {
		err := filepath.Walk(ld, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(ld, path)
			if rel == "." || fi.IsDir() || seen[rel] || reported[rel] {
				return nil
			}
			reported[rel] = true
			out = append(out, FileChange{Kind: "D", Path: "/" + rel})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("runtime.Diff: walk lower: %w", err)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// lowerFile returns the first lower-layer path for rel, or "" if absent.
func lowerFile(lowerDirs []string, rel string) string {
	for _, d := range lowerDirs {
		if p := filepath.Join(d, rel); pathExists(p) {
			return p
		}
	}
	return ""
}

// pathExists reports whether Lstat succeeds on path.
func pathExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// fileContentDiffers reports whether two paths differ in type or content
// (regular files by SHA-256, symlinks by target).
func fileContentDiffers(a, b string) bool {
	fa, errA := os.Lstat(a)
	fb, errB := os.Lstat(b)
	if errA != nil || errB != nil {
		return true
	}
	if fa.Mode()&os.ModeSymlink != 0 || fb.Mode()&os.ModeSymlink != 0 {
		ta, errA := os.Readlink(a)
		tb, errB := os.Readlink(b)
		return errA != nil || errB != nil || ta != tb
	}
	if fa.IsDir() || fb.IsDir() {
		return fa.IsDir() != fb.IsDir()
	}
	ha, errA := fileHash(a)
	hb, errB := fileHash(b)
	return errA != nil || errB != nil || ha != hb
}

// fileHash returns the SHA-256 of a regular file.
func fileHash(path string) ([32]byte, error) {
	var zero [32]byte
	f, err := os.Open(path)
	if err != nil {
		return zero, err
	}
	defer f.Close() //nolint:errcheck
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return zero, err
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum, nil
}

// lowerLayerDirs returns the extracted image layer directories for the
// container's image, used to distinguish Changed from Added in Diff.
func lowerLayerDirs(id string) []string {
	data, err := os.ReadFile(filepath.Join(containerDir(id), "config.json"))
	if err != nil {
		return nil
	}
	var cfg ContainerConfig
	if err := json.Unmarshal(data, &cfg); err != nil || cfg.Image == "" {
		return nil
	}
	metaData, err := os.ReadFile(filepath.Join("/var/lib/thrive/images", image.SafeRef(cfg.Image), "manifest.json"))
	if err != nil {
		return nil
	}
	var meta struct {
		Layers []image.Layer
	}
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return nil
	}
	var dirs []string
	for _, l := range meta.Layers {
		if l.Path != "" {
			dirs = append(dirs, l.Path)
		}
	}
	return dirs
}

// lowerExists reports whether rel exists in any lower image layer.
func lowerExists(lowerDirs []string, rel string) bool {
	for _, d := range lowerDirs {
		if _, err := os.Lstat(filepath.Join(d, rel)); err == nil {
			return true
		}
	}
	return false
}

// isOpaqueDir reports whether path carries the OverlayFS opaque xattr.
func isOpaqueDir(path string) bool {
	buf := make([]byte, 8)
	n, err := unix.Getxattr(path, "trusted.overlay.opaque", buf)
	if err != nil {
		return false
	}
	return strings.TrimRight(string(buf[:n]), "\x00") == "y"
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

// CommitOptions holds docker commit parity fields (-a/--author, -m/--message,
// -p/--pause). Author/Message are recorded in the image manifest; Pause
// freezes a running container around the layer copy (best effort).
type CommitOptions struct {
	Author  string
	Message string
	Pause   bool
}

// Commit snapshots the container's writable layer as a new local image.
func Commit(ctx context.Context, id, newRef string) error {
	return CommitWithOptions(ctx, id, newRef, CommitOptions{})
}

// CommitWithOptions snapshots with author/message metadata and optional pause.
func CommitWithOptions(ctx context.Context, id, newRef string, opts CommitOptions) error {
	state, err := loadState(id)
	if err != nil {
		return fmt.Errorf("runtime.Commit: %w", err)
	}
	if newRef == "" {
		return fmt.Errorf("runtime.Commit: image reference required")
	}
	// Overlay mode snapshots the upper dir (delta layer). Copy-fallback
	// mode has no delta: writes live in merged, so snapshot the full
	// rootfs (squashed layer, deletions inherently captured).
	srcDir := filepath.Join(containerDir(id), "upper")
	if mountMode(id) == image.MountModeCopy {
		srcDir = filepath.Join(containerDir(id), "merged")
	}
	if _, err := os.Stat(srcDir); err != nil {
		return fmt.Errorf("runtime.Commit: no writable layer for container %s", id)
	}
	paused := false
	if opts.Pause && state.Status == "running" {
		if err := Pause(ctx, id); err == nil {
			paused = true
		}
	}
	if paused {
		defer Unpause(ctx, id) //nolint:errcheck
	}
	sum := sha256.Sum256([]byte(newRef + id + time.Now().UTC().String()))
	digest := "sha256:" + hex.EncodeToString(sum[:])
	imgDir := filepath.Join("/var/lib/thrive/images", image.SafeRef(newRef))
	layerDir := filepath.Join(imgDir, "layers", digest)
	if err := os.MkdirAll(layerDir, 0755); err != nil {
		return fmt.Errorf("runtime.Commit: mkdir: %w", err)
	}
	if err := copyDir(srcDir, layerDir); err != nil {
		return fmt.Errorf("runtime.Commit: copy: %w", err)
	}
	meta := struct {
		Ref     string
		Digest  string
		Layers  []image.Layer
		Author  string `json:",omitempty"`
		Message string `json:",omitempty"`
		Created string `json:",omitempty"`
	}{
		Ref:     newRef,
		Digest:  digest,
		Layers:  []image.Layer{{Digest: digest, Path: layerDir}},
		Author:  opts.Author,
		Message: opts.Message,
		Created: time.Now().UTC().Format(time.RFC3339),
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

// ParseRestartPolicy parses docker-style restart specs: no, always,
// on-failure[:max], unless-stopped.
func ParseRestartPolicy(spec string) (RestartPolicy, error) {
	if spec == "" || spec == "no" {
		return RestartPolicy{Name: "no"}, nil
	}
	name, maxStr, _ := strings.Cut(spec, ":")
	max := 0
	if maxStr != "" {
		var err error
		max, err = strconv.Atoi(maxStr)
		if err != nil || max < 0 {
			return RestartPolicy{}, fmt.Errorf("runtime: invalid restart max %q", maxStr)
		}
	}
	switch name {
	case "always", "on-failure", "unless-stopped":
		return RestartPolicy{Name: name, MaxRetryAttempts: max}, nil
	default:
		return RestartPolicy{}, fmt.Errorf("runtime: invalid restart policy %q (no, always, on-failure[:max], unless-stopped)", spec)
	}
}

// tryRestart relaunches an exited container per its restart policy.
// Returns true when a restart was kicked off (caller must stop there).
func tryRestart(ctx context.Context, id, containerDir string, state *ContainerState, cfg *ContainerConfig, exitCode int, log *zap.Logger) bool {
	policy := ""
	maxRetries := 0
	if cfg != nil {
		policy = cfg.RestartPolicy.Name
		maxRetries = cfg.RestartPolicy.MaxRetryAttempts
	}
	// Count prior restarts from the state file.
	restarts := 0
	if data, err := os.ReadFile(filepath.Join(containerDir, "restarts")); err == nil {
		restarts, _ = strconv.Atoi(strings.TrimSpace(string(data)))
	}
	should := false
	switch policy {
	case "always", "unless-stopped":
		should = true
	case "on-failure":
		should = exitCode != 0
	}
	if !should {
		return false
	}
	if maxRetries > 0 && restarts >= maxRetries {
		log.Info("runtime: restart limit reached", telemetry.FieldString("containerID", id))
		return false
	}
	restarts++
	os.WriteFile(filepath.Join(containerDir, "restarts"), []byte(strconv.Itoa(restarts)), 0644) //nolint:errcheck
	log.Info("runtime: restarting container",
		telemetry.FieldString("containerID", id), telemetry.FieldInt("attempt", restarts))
	events.Log("container", "restart", id, map[string]string{"attempt": strconv.Itoa(restarts)})

	state.Status = "created"
	state.PID = 0
	if err := saveState(containerDir, state); err != nil {
		log.Error("runtime: restart saveState failed", telemetry.FieldError(err))
		return false
	}
	image.Unmount(ctx, id) //nolint:errcheck
	if _, err := Start(ctx, id); err != nil {
		log.Error("runtime: restart Start failed", telemetry.FieldError(err))
		state.Status = "stopped"
		state.ExitCode = exitCode
		saveState(containerDir, state) //nolint:errcheck
		return false
	}
	return true
}
