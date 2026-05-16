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

// Start launches the VM and waits for it to be ready
func Start(ctx context.Context, cfg *Config) error {
	switch cfg.VMType {
	case "darwin-hv":
		return startDarwinHV(ctx, cfg)
	case "hyperv":
		return startHyperV(ctx, cfg)
	case "wsl2":
		return startWSL2(ctx, cfg)
	default:
		return fmt.Errorf("unsupported vm type: %s", cfg.VMType)
	}
}

// Stop gracefully stops the VM
func Stop(ctx context.Context) error {
	state, err := ReadVMState()
	if err != nil {
		return err
	}

	if !state.Running {
		return nil
	}

	switch state.VMType {
	case "darwin-hv":
		return stopDarwinHV(ctx, state)
	case "hyperv":
		return stopHyperV(ctx, state)
	case "wsl2":
		return stopWSL2(ctx, state)
	default:
		return fmt.Errorf("unknown vm type in state")
	}
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