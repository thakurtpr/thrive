//go:build !linux

package vm

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

const wslDistroName = "Thrive"

// wsl2Launcher registers and starts the Thrive WSL2 distro on Windows.
// All external interactions (exec, lookup, filesystem) are injected as
// function fields so tests can verify the wsl.exe argument contract.
type wsl2Launcher struct {
	runner              commandRunner
	lookPath            pathLookup
	distroAlreadyExists func(ctx context.Context, name string) bool
	rootfsPath          func() string
	instanceDir         func() string
}

func newWSL2Launcher() *wsl2Launcher {
	l := &wsl2Launcher{
		runner:      realCommandRunner,
		lookPath:    exec.LookPath,
		rootfsPath:  func() string { return filepath.Join(ThriveDir(), "vm", "rootfs.tar.gz") },
		instanceDir: func() string { return filepath.Join(ThriveDir(), "vm", "wsl") },
	}
	l.distroAlreadyExists = func(ctx context.Context, name string) bool {
		out, err := l.runner(ctx, "wsl.exe", "--list", "--quiet")
		if err != nil {
			return false
		}
		// wsl --list --quiet output may be UTF-16LE on older builds;
		// tolerate by lowercasing and checking substring.
		return strings.Contains(strings.ToLower(string(out)), strings.ToLower(name))
	}
	return l
}

func (l *wsl2Launcher) Start(ctx context.Context, cfg *Config) (*VMState, error) {
	binPath, err := l.lookPath("wsl.exe")
	if err != nil {
		return nil, fmt.Errorf("wsl.exe not found in PATH — enable WSL2 (Windows Features) and install Linux kernel update: %w", err)
	}

	if !l.distroAlreadyExists(ctx, wslDistroName) {
		out, err := l.runner(ctx, binPath,
			"--import", wslDistroName, l.instanceDir(), l.rootfsPath(),
			"--version", "2",
		)
		if err != nil {
			return nil, fmt.Errorf("wsl --import failed: %w: %s", err, string(out))
		}
	}

	// Boot the distro so the in-VM thrive-daemon comes up. Non-zero exit
	// here is non-fatal: the subsequent bridge dial validates readiness.
	_, _ = l.runner(ctx, binPath, "-d", wslDistroName, "--exec", "/sbin/init")

	return &VMState{
		Version:     "1.0",
		Running:     true,
		VMType:      "wsl2",
		WSLInstance: wslDistroName,
	}, nil
}

func (l *wsl2Launcher) Stop(ctx context.Context, state *VMState) error {
	if state == nil || !state.Running {
		return nil
	}
	binPath, err := l.lookPath("wsl.exe")
	if err != nil {
		return fmt.Errorf("wsl.exe not found: %w", err)
	}
	name := state.WSLInstance
	if name == "" {
		name = wslDistroName
	}
	if out, err := l.runner(ctx, binPath, "--terminate", name); err != nil {
		return fmt.Errorf("wsl --terminate failed: %w: %s", err, string(out))
	}
	return nil
}
