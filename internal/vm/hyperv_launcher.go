//go:build windows

package vm

import (
	"context"
	"fmt"
	"os"
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
	vhdExists       func(path string) bool
	prepareListener func(port uint32) error
}

func newHyperVLauncher() *hyperVLauncher {
	l := &hyperVLauncher{
		runner:   realCommandRunner,
		lookPath: exec.LookPath,
		vhdPath:  func() string { return filepath.Join(ThriveDir(), "vm", "disk.vhdx") },
		vhdExists: func(path string) bool {
			_, err := os.Stat(path)
			return err == nil
		},
		prepareListener: PrepareHVSOCKListener,
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
		// vhdExists is nil in tests that construct hyperVLauncher directly
		// without newHyperVLauncher() — real callers always go through the
		// constructor, so the check is always enforced in production.
		if l.vhdExists != nil && !l.vhdExists(l.vhdPath()) {
			return nil, fmt.Errorf("VM image not found at %s — run 'thrive desktop init' first", l.vhdPath())
		}

		script := fmt.Sprintf(
			"New-VM -Name '%s' -MemoryStartupBytes %dMB -Generation 2 -VHDPath '%s' -SwitchName 'Default Switch'; "+
				"Set-VMProcessor -VMName '%s' -Count %d; "+
				"Set-VMFirmware -VMName '%s' -EnableSecureBoot Off",
			hyperVVMName, cfg.MemoryMB, l.vhdPath(), hyperVVMName, cfg.CPUCount, hyperVVMName,
		)
		if out, err := l.runner(ctx, binPath, "-NoProfile", "-Command", script); err != nil {
			return nil, fmt.Errorf("New-VM failed: %w: %s", err, string(out))
		}
	}

	// Listener must exist before Start-VM: the guest connects out within
	// ~1s of boot, and a missing listener at that moment means the daemon
	// falls back to its reconnect loop, delaying (not breaking) detection.
	if l.prepareListener != nil {
		if err := l.prepareListener(vsockPort()); err != nil {
			return nil, fmt.Errorf("hvsock listener: %w", err)
		}
	}

	if out, err := l.runner(ctx, binPath, "-NoProfile", "-Command",
		fmt.Sprintf("Start-VM -Name '%s'", hyperVVMName)); err != nil {
		return nil, fmt.Errorf("Start-VM failed: %w: %s", err, string(out))
	}

	vmID := ""
	if out, err := l.runner(ctx, binPath, "-NoProfile", "-Command",
		fmt.Sprintf("(Get-VM -Name '%s').VMId.Guid", hyperVVMName)); err == nil {
		vmID = strings.TrimSpace(string(out))
	}

	return &VMState{
		Version: "1.0",
		Running: true,
		VMType:  "hyperv",
		VMID:    vmID,
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
	CloseHVSOCKListener()
	return nil
}
