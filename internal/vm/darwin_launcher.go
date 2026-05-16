//go:build !linux

package vm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// darwinLauncher boots a Linux VM on macOS using vfkit, an Apple
// Virtualization.framework CLI wrapper. Function-field injection allows
// tests to assert subprocess contracts without spawning real processes.
type darwinLauncher struct {
	starter     processStarter
	lookPath    pathLookup
	killProcess func(pid int) error
}

func newDarwinLauncher() *darwinLauncher {
	return &darwinLauncher{
		starter:     realProcessStarter,
		lookPath:    exec.LookPath,
		killProcess: defaultKillProcess,
	}
}

func defaultKillProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGTERM)
}

func (l *darwinLauncher) Start(ctx context.Context, cfg *Config) (*VMState, error) {
	vmDir := filepath.Join(ThriveDir(), "vm")

	// Prefer the bundled vfkit shipped inside the VM image tarball by `thrive desktop init`.
	binPath := filepath.Join(vmDir, "vfkit")
	if _, err := os.Stat(binPath); err != nil {
		var pathErr error
		binPath, pathErr = l.lookPath("vfkit")
		if pathErr != nil {
			return nil, fmt.Errorf(
				"vfkit not found — run `thrive desktop init` to download it: %w", pathErr)
		}
	}

	kernel := filepath.Join(vmDir, "kernel")
	initrd := filepath.Join(vmDir, "initrd.img")

	args := []string{
		"--memory", strconv.Itoa(cfg.MemoryMB),
		"--cpus", strconv.Itoa(cfg.CPUCount),
		"--bootloader", fmt.Sprintf("linux,kernel=%s,initrd=%s,cmdline=\"console=hvc0 quiet\"", kernel, initrd),
		"--device", "virtio-rng",
		"--device", "virtio-vsock,port=1024,socketURL=" + filepath.Join(vmDir, "vsock.sock"),
	}

	pid, err := l.starter(binPath, args...)
	if err != nil {
		return nil, fmt.Errorf("vfkit spawn failed: %w", err)
	}

	return &VMState{
		Version: "1.0",
		Running: true,
		PID:     pid,
		VMType:  "darwin-hv",
	}, nil
}

func (l *darwinLauncher) Stop(ctx context.Context, state *VMState) error {
	if state == nil || !state.Running {
		return nil
	}
	if err := l.killProcess(state.PID); err != nil {
		return fmt.Errorf("failed to signal vfkit pid %d: %w", state.PID, err)
	}
	return nil
}
