//go:build linux
// +build linux

package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// Create initializes a new container from the given config but does not start it.
func Create(ctx context.Context, cfg ContainerConfig) (*Container, error) {
	if cfg.ID == "" {
		return nil, fmt.Errorf("runtime.Create: container ID required")
	}

	containerDir := filepath.Join("/run/thrive/containers", cfg.ID)
	if err := os.MkdirAll(containerDir, 0755); err != nil {
		return nil, fmt.Errorf("runtime.Create: mkdir %s: %w", containerDir, err)
	}

	// Save config for Start to use
	configPath := filepath.Join(containerDir, "config.json")
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("runtime.Create: marshal config: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return nil, fmt.Errorf("runtime.Create: write config: %w", err)
	}

	container := &Container{
		ID:     cfg.ID,
		Config: &cfg,
		State: &ContainerState{
			ID:     cfg.ID,
			Status: "created",
		},
	}

	if err := saveState(containerDir, container.State); err != nil {
		return nil, fmt.Errorf("runtime.Create: saveState: %w", err)
	}

	return container, nil
}

// Start executes the container's main process with namespace isolation.
func Start(ctx context.Context, id string) error {
	state, err := loadState(id)
	if err != nil {
		return fmt.Errorf("runtime.Start: %w", err)
	}

	if state.Status != "created" {
		return fmt.Errorf("runtime.Start: container already started or deleted")
	}

	// Get container config
	configPath := filepath.Join("/run/thrive/containers", id, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("runtime.Start: read config: %w", err)
	}
	cfg := &ContainerConfig{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("runtime.Start: unmarshal config: %w", err)
	}

	// Build command
	cmd := cfg.Command
	if len(cmd) == 0 {
		cmd = []string{"/bin/sh"}
	}

	// Prepare exec.Command with namespace flags
	execCmd := exec.Command(cmd[0], cmd[1:]...)
	execCmd.Args = cmd
	execCmd.Env = cfg.Env
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	// Set up namespace flags
	execCmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC,
		UidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getuid(), Size: 1},
		},
		GidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getgid(), Size: 1},
		},
	}

	// Run the command
	err = execCmd.Start()
	if err != nil {
		return fmt.Errorf("runtime.Start: exec: %w", err)
	}

	pid := execCmd.Process.Pid

	// Wait for the process
	var status syscall.WaitStatus
	syscall.Wait4(pid, &status, 0, nil)

	// Update state
	state.PID = pid
	state.Status = "running"
	if status.Exited() {
		state.Status = "stopped"
		state.ExitCode = status.ExitStatus()
	}
	saveState(filepath.Join("/run/thrive/containers", id), state)

	return nil
}

// Kill sends a signal to the container's main process.
func Kill(ctx context.Context, id string, signal syscall.Signal) error {
	state, err := loadState(id)
	if err != nil {
		return fmt.Errorf("runtime.Kill: %w", err)
	}

	if state.PID <= 0 {
		return fmt.Errorf("runtime.Kill: container not running")
	}

	if err := syscall.Kill(state.PID, signal); err != nil {
		return fmt.Errorf("runtime.Kill: kill: %w", err)
	}

	return nil
}

// Delete removes the container's state and resources.
func Delete(ctx context.Context, id string) error {
	containerDir := filepath.Join("/run/thrive/containers", id)
	if err := os.RemoveAll(containerDir); err != nil {
		return fmt.Errorf("runtime.Delete: remove %s: %w", containerDir, err)
	}
	return nil
}

// State returns the current state of a container.
func State(ctx context.Context, id string) (*ContainerState, error) {
	return loadState(id)
}

func loadState(id string) (*ContainerState, error) {
	statePath := filepath.Join("/run/thrive/containers", id, "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, fmt.Errorf("loadState: read %s: %w", statePath, err)
	}

	state := &ContainerState{}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("loadState: unmarshal: %w", err)
	}

	return state, nil
}

func saveState(dir string, state *ContainerState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "state.json"), data, 0644)
}
