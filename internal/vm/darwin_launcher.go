//go:build darwin

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
	starter         processStarter
	lookPath        pathLookup
	killProcess     func(pid int) error
	prepareListener func() error
}

func newDarwinLauncher() *darwinLauncher {
	return &darwinLauncher{
		starter:         realProcessStarter,
		lookPath:        exec.LookPath,
		killProcess:     defaultKillProcess,
		prepareListener: PrepareVSOCKListener,
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

	serialLog := filepath.Join(vmDir, "vm-serial.log")

	// Host image store shared with VM via virtiofs
	home, _ := os.UserHomeDir()
	hostImages := filepath.Join(home, ".thrive", "images")
	os.MkdirAll(hostImages, 0755)

	args := []string{
		"--memory", strconv.Itoa(cfg.MemoryMB),
		"--cpus", strconv.Itoa(cfg.CPUCount),
		// ip=dhcp: kernel configures eth0 via DHCP at boot
		"--bootloader", fmt.Sprintf("linux,kernel=%s,initrd=%s,cmdline=\"console=hvc0 console=ttyS0 ip=dhcp\"", kernel, initrd),
		"--device", "virtio-rng",
		"--device", "virtio-net,nat",
		// virtiofs: shares ~/.thrive/images/ → /var/lib/thrive/images/ in VM
		"--device", "virtio-fs,sharedDir=" + hostImages + ",mountTag=thrive-images",
		"--device", "virtio-vsock,port=1024,socketURL=" + filepath.Join(vmDir, "vsock.sock"),
		"--device", "virtio-serial,logFilePath=" + serialLog,
	}

	prepare := l.prepareListener
	if prepare == nil {
		prepare = PrepareVSOCKListener
	}
	if err := prepare(); err != nil {
		return nil, err
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
