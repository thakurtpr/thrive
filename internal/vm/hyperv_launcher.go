//go:build !linux

package vm

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

const hyperVVMName = "Thrive"

// hyperVLauncher creates and starts a Hyper-V Generation 2 VM by shelling
// out to PowerShell's Hyper-V module. Function-field injection allows
// tests to assert the cmdlet contract without launching real VMs.
type hyperVLauncher struct {
	runner          commandRunner
	lookPath        pathLookup
	vmAlreadyExists func(ctx context.Context, name string) bool
	vhdPath         func() string
}

func newHyperVLauncher() *hyperVLauncher {
	l := &hyperVLauncher{
		runner:   realCommandRunner,
		lookPath: exec.LookPath,
		vhdPath:  func() string { return filepath.Join(ThriveDir(), "vm", "disk.vhdx") },
	}
	l.vmAlreadyExists = func(ctx context.Context, name string) bool {
		out, err := l.runner(ctx, "powershell.exe", "-NoProfile", "-Command",
			fmt.Sprintf("if (Get-VM -Name '%s' -ErrorAction SilentlyContinue) { 'yes' } else { 'no' }", name))
		if err != nil {
			return false
		}
		return strings.Contains(string(out), "yes")
	}
	return l
}

func (l *hyperVLauncher) Start(ctx context.Context, cfg *Config) (*VMState, error) {
	binPath, err := l.lookPath("powershell.exe")
	if err != nil {
		return nil, fmt.Errorf("powershell.exe not found in PATH — enable Hyper-V via Windows Features: %w", err)
	}

	if !l.vmAlreadyExists(ctx, hyperVVMName) {
		script := fmt.Sprintf(
			"New-VM -Name '%s' -MemoryStartupBytes %dMB -Generation 2 -VHDPath '%s' -SwitchName 'Default Switch'; "+
				"Set-VMProcessor -VMName '%s' -Count %d",
			hyperVVMName, cfg.MemoryMB, l.vhdPath(), hyperVVMName, cfg.CPUCount,
		)
		if out, err := l.runner(ctx, binPath, "-NoProfile", "-Command", script); err != nil {
			return nil, fmt.Errorf("New-VM failed: %w: %s", err, string(out))
		}
	}

	if out, err := l.runner(ctx, binPath, "-NoProfile", "-Command",
		fmt.Sprintf("Start-VM -Name '%s'", hyperVVMName)); err != nil {
		return nil, fmt.Errorf("Start-VM failed: %w: %s", err, string(out))
	}

	return &VMState{
		Version: "1.0",
		Running: true,
		VMType:  "hyperv",
	}, nil
}

func (l *hyperVLauncher) Stop(ctx context.Context, state *VMState) error {
	if state == nil || !state.Running {
		return nil
	}
	binPath, err := l.lookPath("powershell.exe")
	if err != nil {
		return fmt.Errorf("powershell.exe not found: %w", err)
	}
	if out, err := l.runner(ctx, binPath, "-NoProfile", "-Command",
		fmt.Sprintf("Stop-VM -Name '%s' -Force", hyperVVMName)); err != nil {
		return fmt.Errorf("Stop-VM failed: %w: %s", err, string(out))
	}
	return nil
}
