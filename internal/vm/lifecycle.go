//go:build !linux

package vm

import (
	"context"
	"fmt"
	"log"
	"time"
)

const (
	bootTimeoutSeconds = 30
	bootPollInterval   = 1 * time.Second
)

// launcher is the platform-agnostic VM lifecycle contract.
type launcher interface {
	Start(ctx context.Context, cfg *Config) (*VMState, error)
	Stop(ctx context.Context, state *VMState) error
}

func selectLauncher(vmType string) (launcher, error) {
	switch vmType {
	case "darwin-hv":
		return newDarwinLauncher(), nil
	case "hyperv":
		return newHyperVLauncher(), nil
	case "wsl2":
		return newWSL2Launcher(), nil
	default:
		return nil, fmt.Errorf("unsupported vm type: %s", vmType)
	}
}

// Start launches the VM and persists the resulting state.
func Start(ctx context.Context, cfg *Config) error {
	l, err := selectLauncher(cfg.VMType)
	if err != nil {
		return err
	}
	state, err := l.Start(ctx, cfg)
	if err != nil {
		return err
	}
	return WriteVMState(state)
}

// Stop gracefully stops the VM and updates persisted state.
func Stop(ctx context.Context) error {
	state, err := ReadVMState()
	if err != nil {
		return err
	}
	if !state.Running {
		return nil
	}
	l, err := selectLauncher(state.VMType)
	if err != nil {
		return err
	}
	if err := l.Stop(ctx, state); err != nil {
		return err
	}
	state.Running = false
	return WriteVMState(state)
}

// WaitForBoot blocks until the VM is reachable via the bridge
func WaitForBoot(ctx context.Context, cfg *Config) error {
	bridge, err := Dial(ctx, cfg.VMType)
	if err != nil {
		return err
	}
	defer bridge.Close()

	for i := 0; i < bootTimeoutSeconds; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if _, err := bridge.Exec(ctx, "ping", nil, nil); err == nil {
			log.Printf("vm: boot complete after %d seconds", i)
			return nil
		}

		time.Sleep(bootPollInterval)
	}

	return fmt.Errorf("vm: boot timeout after %d seconds", bootTimeoutSeconds)
}

// HealthCheck pings the VM daemon and returns nil if it's responsive
func HealthCheck(ctx context.Context, vmType string) error {
	bridge, err := Dial(ctx, vmType)
	if err != nil {
		return err
	}
	defer bridge.Close()

	_, err = bridge.Exec(ctx, "ping", nil, nil)
	return err
}