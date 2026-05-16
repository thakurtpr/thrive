//go:build !linux

package vm

import (
	"context"
	"os"
	"os/exec"
)

// commandRunner runs a command to completion and returns combined output.
// Used for synchronous one-shot commands (wsl --terminate, powershell Stop-VM).
type commandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

// processStarter starts a long-running detached process and returns its PID.
// Used for foreground hypervisor processes (vfkit, qemu) that must outlive
// the `thrive desktop start` invocation. No ctx binding by design.
type processStarter func(name string, args ...string) (pid int, err error)

// pathLookup finds an executable in PATH. Injectable for tests.
type pathLookup func(file string) (string, error)

func realCommandRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func realProcessStarter(name string, args ...string) (int, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	// Release so the child is not reparented as a zombie when thrive exits.
	_ = cmd.Process.Release()
	return pid, nil
}
