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

	"github.com/thakurprasadrout/thrive/internal/secrets"
	"github.com/thakurprasadrout/thrive/internal/telemetry"
)

// Create initializes a new container from the given config but does not start it.
func Create(ctx context.Context, cfg ContainerConfig) (*Container, error) {
	log := telemetry.Logger()
	log.Info("runtime.Create: starting",
		telemetry.FieldString("containerID", cfg.ID),
		telemetry.FieldString("image", cfg.Image))

	if cfg.ID == "" {
		log.Error("runtime.Create: container ID required", telemetry.FieldString("error", "empty ID"))
		return nil, fmt.Errorf("runtime.Create: container ID required")
	}
	telemetry.Debug("runtime.Create: container ID validated", telemetry.FieldString("containerID", cfg.ID))

	containerDir := filepath.Join("/run/thrive/containers", cfg.ID)
	log.Info("runtime.Create: creating container directory", telemetry.FieldString("path", containerDir))

	if err := os.MkdirAll(containerDir, 0755); err != nil {
		log.Error("runtime.Create: mkdir failed", telemetry.FieldString("path", containerDir), telemetry.FieldError(err))
		return nil, fmt.Errorf("runtime.Create: mkdir %s: %w", containerDir, err)
	}
	telemetry.Debug("runtime.Create: container directory created", telemetry.FieldString("path", containerDir))

	// Save config for Start to use
	configPath := filepath.Join(containerDir, "config.json")
	log.Info("runtime.Create: marshaling config", telemetry.FieldString("configPath", configPath))

	data, err := json.Marshal(cfg)
	if err != nil {
		log.Error("runtime.Create: marshal config failed", telemetry.FieldError(err))
		return nil, fmt.Errorf("runtime.Create: marshal config: %w", err)
	}
	telemetry.Debug("runtime.Create: config marshaled", telemetry.FieldInt("size", len(data)))

	log.Info("runtime.Create: writing config file", telemetry.FieldString("configPath", configPath))
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		log.Error("runtime.Create: write config failed", telemetry.FieldString("configPath", configPath), telemetry.FieldError(err))
		return nil, fmt.Errorf("runtime.Create: write config: %w", err)
	}
	telemetry.Debug("runtime.Create: config file written", telemetry.FieldString("configPath", configPath))

	container := &Container{
		ID:     cfg.ID,
		Config: &cfg,
		State: &ContainerState{
			ID:     cfg.ID,
			Status: "created",
		},
	}
	telemetry.Debug("runtime.Create: container struct created", telemetry.FieldString("containerID", container.ID))

	log.Info("runtime.Create: saving initial state", telemetry.FieldString("containerID", cfg.ID))
	if err := saveState(containerDir, container.State); err != nil {
		log.Error("runtime.Create: saveState failed", telemetry.FieldString("containerID", cfg.ID), telemetry.FieldError(err))
		return nil, fmt.Errorf("runtime.Create: saveState: %w", err)
	}
	telemetry.Debug("runtime.Create: state saved", telemetry.FieldString("containerID", cfg.ID))

	log.Info("runtime.Create: completed successfully", telemetry.FieldString("containerID", cfg.ID))
	return container, nil
}

// Start executes the container's main process with namespace isolation.
func Start(ctx context.Context, id string) error {
	log := telemetry.Logger()
	log.Info("runtime.Start: starting", telemetry.FieldString("containerID", id))

	state, err := loadState(id)
	if err != nil {
		log.Error("runtime.Start: loadState failed", telemetry.FieldString("containerID", id), telemetry.FieldError(err))
		return fmt.Errorf("runtime.Start: %w", err)
	}
	telemetry.Debug("runtime.Start: state loaded", telemetry.FieldString("containerID", id), telemetry.FieldString("status", state.Status))

	if state.Status != "created" {
		log.Warn("runtime.Start: container not in created state",
			telemetry.FieldString("containerID", id),
			telemetry.FieldString("status", state.Status))
		return fmt.Errorf("runtime.Start: container already started or deleted")
	}
	telemetry.Debug("runtime.Start: status check passed", telemetry.FieldString("containerID", id))

	// Get container config
	configPath := filepath.Join("/run/thrive/containers", id, "config.json")
	log.Info("runtime.Start: reading config file", telemetry.FieldString("configPath", configPath))

	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Error("runtime.Start: read config failed", telemetry.FieldString("configPath", configPath), telemetry.FieldError(err))
		return fmt.Errorf("runtime.Start: read config: %w", err)
	}
	telemetry.Debug("runtime.Start: config file read", telemetry.FieldInt("size", len(data)))

	cfg := &ContainerConfig{}
	if err := json.Unmarshal(data, cfg); err != nil {
		log.Error("runtime.Start: unmarshal config failed", telemetry.FieldError(err))
		return fmt.Errorf("runtime.Start: unmarshal config: %w", err)
	}
	telemetry.Debug("runtime.Start: config unmarshaled", telemetry.FieldString("image", cfg.Image))

	// Build command
	cmd := cfg.Command
	if len(cmd) == 0 {
		cmd = []string{"/bin/sh"}
		telemetry.Debug("runtime.Start: using default command", telemetry.FieldString("command", "/bin/sh"))
	}
	telemetry.Debug("runtime.Start: command prepared", telemetry.FieldString("command", cmd[0]), telemetry.FieldInt("args", len(cmd)))

	log.Info("runtime.Start: preparing exec.Command", telemetry.FieldString("command", cmd[0]))
	execCmd := exec.Command(cmd[0], cmd[1:]...)
	execCmd.Args = cmd
	execCmd.Env = cfg.Env
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	// Set up namespace flags
	log.Info("runtime.Start: setting up namespace flags",
		telemetry.FieldInt("uid", os.Getuid()),
		telemetry.FieldInt("gid", os.Getgid()))

	execCmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC,
		UidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getuid(), Size: 1},
		},
		GidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getgid(), Size: 1},
		},
	}
	telemetry.Debug("runtime.Start: SysProcAttr configured", telemetry.FieldInt("cloneflags", syscall.CLONE_NEWPID|syscall.CLONE_NEWNS|syscall.CLONE_NEWUTS|syscall.CLONE_NEWIPC))

	if len(cfg.Secrets) > 0 {
		log.Info("runtime.Start: injecting secrets", telemetry.FieldString("containerID", id), telemetry.FieldInt("count", len(cfg.Secrets)))
		if err := secrets.Inject(id, cfg.Secrets); err != nil {
			log.Error("runtime.Start: secrets.Inject failed", telemetry.FieldString("containerID", id), telemetry.FieldError(err))
		}
	}

	log.Info("runtime.Start: starting process")
	err = execCmd.Start()
	if err != nil {
		log.Error("runtime.Start: exec.Start failed", telemetry.FieldError(err))
		return fmt.Errorf("runtime.Start: exec: %w", err)
	}

	pid := execCmd.Process.Pid
	log.Info("runtime.Start: process started", telemetry.FieldInt("pid", pid))

	log.Info("runtime.Start: waiting for process")
	var status syscall.WaitStatus
	syscall.Wait4(pid, &status, 0, nil)
	telemetry.Debug("runtime.Start: process exited", telemetry.FieldInt("exitStatus", status.ExitStatus()))

	// Update state
	state.PID = pid
	state.Status = "running"
	if status.Exited() {
		state.Status = "stopped"
		state.ExitCode = status.ExitStatus()
		log.Info("runtime.Start: process exited with code", telemetry.FieldInt("exitCode", state.ExitCode))
	}
	telemetry.Debug("runtime.Start: updating state",
		telemetry.FieldInt("pid", pid),
		telemetry.FieldString("status", state.Status))

	saveState(filepath.Join("/run/thrive/containers", id), state)

	if len(cfg.Secrets) > 0 {
		log.Info("runtime.Start: cleaning up secrets", telemetry.FieldString("containerID", id))
		if err := secrets.Cleanup(id); err != nil {
			log.Warn("runtime.Start: secrets.Cleanup failed", telemetry.FieldString("containerID", id), telemetry.FieldError(err))
		}
	}

	log.Info("runtime.Start: completed", telemetry.FieldString("containerID", id), telemetry.FieldString("finalStatus", state.Status))

	return nil
}

// Kill sends a signal to the container's main process.
func Kill(ctx context.Context, id string, signal syscall.Signal) error {
	log := telemetry.Logger()
	log.Info("runtime.Kill: starting", telemetry.FieldString("containerID", id), telemetry.FieldInt("signal", int(signal)))

	state, err := loadState(id)
	if err != nil {
		log.Error("runtime.Kill: loadState failed", telemetry.FieldString("containerID", id), telemetry.FieldError(err))
		return fmt.Errorf("runtime.Kill: %w", err)
	}

	if state.PID <= 0 {
		log.Warn("runtime.Kill: container not running", telemetry.FieldString("containerID", id))
		return fmt.Errorf("runtime.Kill: container not running")
	}
	telemetry.Debug("runtime.Kill: target PID", telemetry.FieldInt("pid", state.PID))

	log.Info("runtime.Kill: sending signal", telemetry.FieldInt("pid", state.PID), telemetry.FieldInt("signal", int(signal)))
	if err := syscall.Kill(state.PID, signal); err != nil {
		log.Error("runtime.Kill: kill syscall failed", telemetry.FieldInt("pid", state.PID), telemetry.FieldError(err))
		return fmt.Errorf("runtime.Kill: kill: %w", err)
	}

	log.Info("runtime.Kill: completed successfully", telemetry.FieldString("containerID", id))
	return nil
}

// Delete removes the container's state and resources.
func Delete(ctx context.Context, id string) error {
	log := telemetry.Logger()
	log.Info("runtime.Delete: starting", telemetry.FieldString("containerID", id))

	containerDir := filepath.Join("/run/thrive/containers", id)
	log.Info("runtime.Delete: removing container directory", telemetry.FieldString("path", containerDir))

	if err := os.RemoveAll(containerDir); err != nil {
		log.Error("runtime.Delete: RemoveAll failed", telemetry.FieldString("path", containerDir), telemetry.FieldError(err))
		return fmt.Errorf("runtime.Delete: remove %s: %w", containerDir, err)
	}

	log.Info("runtime.Delete: completed successfully", telemetry.FieldString("containerID", id))
	return nil
}

// State returns the current state of a container.
func State(ctx context.Context, id string) (*ContainerState, error) {
	log := telemetry.Logger()
	log.Debug("runtime.State: loading state", telemetry.FieldString("containerID", id))

	state, err := loadState(id)
	if err != nil {
		log.Error("runtime.State: loadState failed", telemetry.FieldString("containerID", id), telemetry.FieldError(err))
		return nil, err
	}

	log.Debug("runtime.State: state loaded", telemetry.FieldString("containerID", id), telemetry.FieldString("status", state.Status))
	return state, nil
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
