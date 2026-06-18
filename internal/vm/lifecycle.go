//go:build !linux

package vm

import (
	"context"
	"fmt"
	"log"
	"time"
)

const bootTimeout = 2 * time.Minute

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

// WaitForBoot blocks until the VM daemon is reachable and returns the live
// bridge to thrived. The caller MUST close the bridge when done.
func WaitForBoot(ctx context.Context, cfg *Config) (Bridge, error) {
	deadline := time.Now().Add(bootTimeout)
	connCount := 0
	acceptTimeouts := 0
	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("vm: boot timeout after %s (connections: %d, accept-timeouts: %d)",
				bootTimeout, connCount, acceptTimeouts)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		bridge, err := Dial(ctx, cfg.VMType)
		if err != nil {
			acceptTimeouts++
			if acceptTimeouts == 1 || acceptTimeouts%6 == 0 {
				log.Printf("vm: waiting for thrived (%d timeouts, %d connections)...", acceptTimeouts, connCount)
			}
			continue
		}

		connCount++
		log.Printf("vm: connection #%d accepted — pinging...", connCount)

		_, pingErr := bridge.Exec(ctx, "ping", nil, nil)
		if pingErr == nil {
			log.Printf("vm: boot complete")
			return bridge, nil // return the live bridge — do NOT close it
		}
		bridge.Close() // wrong connection (probe); close and retry
		log.Printf("vm: ping #%d failed: %v — retrying...", connCount, pingErr)
	}
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