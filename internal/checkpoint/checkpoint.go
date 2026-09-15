//go:build linux
// +build linux

// Package checkpoint manages container checkpoints via CRIU
// (docker checkpoint parity). All operations require the `criu`
// binary on PATH; otherwise they fail with an explicit error.
package checkpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/thakurprasadrout/thrive/internal/cgroup"
	"github.com/thakurprasadrout/thrive/internal/runtime"
)

// Checkpoint is a saved container checkpoint record.
type Checkpoint struct {
	ContainerID string    `json:"containerID"`
	Name        string    `json:"name"`
	Created     time.Time `json:"created"`
	Dir         string    `json:"dir"`
}

var checkpointsDirOverride string

func baseDir() string {
	if checkpointsDirOverride != "" {
		return checkpointsDirOverride
	}
	return "/var/lib/thrive/checkpoints"
}

func checkpointDir(containerID, name string) string {
	return filepath.Join(baseDir(), containerID, name)
}

// criuAvailable reports whether the criu binary is on PATH.
func criuAvailable() bool {
	_, err := exec.LookPath("criu")
	return err == nil
}

func validName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			c == '_' || c == '.' || c == '-') {
			return false
		}
	}
	return true
}

// Create dumps a running container's process tree with CRIU.
func Create(containerID, name string) (*Checkpoint, error) {
	if !validName(name) {
		return nil, fmt.Errorf("checkpoint: invalid name %q", name)
	}
	if !criuAvailable() {
		return nil, fmt.Errorf("checkpoint: criu binary not found on PATH")
	}
	state, err := runtime.State(context.Background(), containerID)
	if err != nil {
		return nil, fmt.Errorf("checkpoint: %w", err)
	}
	if state.Status != "running" || state.PID <= 0 {
		return nil, fmt.Errorf("checkpoint: container %s is not running", containerID)
	}
	dir := checkpointDir(containerID, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("checkpoint: mkdir: %w", err)
	}
	cmd := exec.Command("criu", "dump", "-t", fmt.Sprintf("%d", state.PID),
		"-D", dir, "--shell-job", "--tcp-established")
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir) //nolint:errcheck
		return nil, fmt.Errorf("checkpoint: criu dump: %w\n%s", err, out)
	}
	cp := &Checkpoint{ContainerID: containerID, Name: name, Created: time.Now().UTC(), Dir: dir}
	data, _ := json.MarshalIndent(cp, "", "  ")
	os.WriteFile(filepath.Join(dir, "checkpoint.json"), data, 0644) //nolint:errcheck
	return cp, nil
}

// List returns checkpoints for a container.
func List(containerID string) ([]*Checkpoint, error) {
	entries, err := os.ReadDir(filepath.Join(baseDir(), containerID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("checkpoint: list: %w", err)
	}
	var out []*Checkpoint
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(baseDir(), containerID, e.Name(), "checkpoint.json"))
		if err != nil {
			continue
		}
		var cp Checkpoint
		if err := json.Unmarshal(data, &cp); err == nil {
			out = append(out, &cp)
		}
	}
	return out, nil
}

// Remove deletes a checkpoint.
func Remove(containerID, name string) error {
	dir := checkpointDir(containerID, name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("checkpoint: %s not found for container %s", name, containerID)
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("checkpoint: remove: %w", err)
	}
	return nil
}

// Restore restores a container from a checkpoint via `criu restore`.
// The container state is flipped back to running with the restored PID.
// cgroup membership is re-applied; network interfaces are NOT rebuilt
// (documented limit: prefer host-network or re-connect after restore).
func Restore(ctx context.Context, containerID, name string) error {
	dir, err := RestoreDir(containerID, name)
	if err != nil {
		return err
	}
	if _, err := runtime.State(ctx, containerID); err != nil {
		return fmt.Errorf("checkpoint: %w", err)
	}
	pidFile := filepath.Join(dir, "restore.pid")
	os.Remove(pidFile) //nolint:errcheck
	cmd := exec.CommandContext(ctx, "criu", "restore",
		"-D", dir, "--shell-job", "--tcp-established",
		"--pidfile", pidFile, "-d")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("checkpoint: criu restore: %w\n%s", err, out)
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("checkpoint: read pidfile: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		return fmt.Errorf("checkpoint: bad pidfile: %w", err)
	}
	statePath := filepath.Join("/run/thrive/containers", containerID, "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return fmt.Errorf("checkpoint: read state: %w", err)
	}
	var state map[string]any
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("checkpoint: parse state: %w", err)
	}
	state["status"] = "running"
	state["pid"] = pid
	updated, _ := json.Marshal(state)
	if err := os.WriteFile(statePath, updated, 0644); err != nil {
		return fmt.Errorf("checkpoint: write state: %w", err)
	}
	if mgr, err := cgroup.New(containerID); err == nil {
		mgr.Apply(pid) //nolint:errcheck
	}
	return nil
}

// RestoreDir returns the CRIU image directory for `start --checkpoint`.
func RestoreDir(containerID, name string) (string, error) {
	if !criuAvailable() {
		return "", fmt.Errorf("checkpoint: criu binary not found on PATH")
	}
	dir := checkpointDir(containerID, name)
	if _, err := os.Stat(filepath.Join(dir, "checkpoint.json")); err != nil {
		return "", fmt.Errorf("checkpoint: %s not found for container %s", name, containerID)
	}
	return dir, nil
}
