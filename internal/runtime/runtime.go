package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// Start executes the container's main process.
func Start(ctx context.Context, id string) error {
	return fmt.Errorf("runtime.Start: not implemented")
}

// Kill sends a signal to the container's main process.
func Kill(ctx context.Context, id string, signal os.Signal) error {
	return fmt.Errorf("runtime.Kill: not implemented")
}

// Delete removes the container's state and resources.
func Delete(ctx context.Context, id string) error {
	return fmt.Errorf("runtime.Delete: not implemented")
}

// State returns the current state of a container.
func State(ctx context.Context, id string) (*ContainerState, error) {
	statePath := filepath.Join("/run/thrive/containers", id, "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, fmt.Errorf("runtime.State: read %s: %w", statePath, err)
	}

	state := &ContainerState{}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("runtime.State: unmarshal: %w", err)
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
