//go:build !linux

package vm

import (
	"context"
	"fmt"
)

// startDarwinHV launches the VM using Darwin HV
func startDarwinHV(ctx context.Context, cfg *Config) error {
	return fmt.Errorf("darwin-hv start not implemented")
}

// stopDarwinHV stops the Darwin HV VM
func stopDarwinHV(ctx context.Context, state *VMState) error {
	return fmt.Errorf("darwin-hv stop not implemented")
}

// startHyperV launches the VM using Hyper-V
func startHyperV(ctx context.Context, cfg *Config) error {
	return fmt.Errorf("hyperv start not implemented")
}

// stopHyperV stops the Hyper-V VM
func stopHyperV(ctx context.Context, state *VMState) error {
	return fmt.Errorf("hyperv stop not implemented")
}

// startWSL2 launches the VM using WSL2
func startWSL2(ctx context.Context, cfg *Config) error {
	return fmt.Errorf("wsl2 start not implemented")
}

// stopWSL2 stops the WSL2 VM
func stopWSL2(ctx context.Context, state *VMState) error {
	return fmt.Errorf("wsl2 stop not implemented")
}
