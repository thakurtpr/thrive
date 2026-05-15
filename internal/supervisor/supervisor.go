//go:build linux
// +build linux

package supervisor

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Policy defines restart behavior when a process exits.
type Policy struct {
	Name       string // "no", "always", "on-failure"
	MaxRetries int
}

// Watch monitors a process and applies restart policy as needed.
func Watch(pid int, policy Policy) error {
	for {
		var status syscall.WaitStatus
		var rusage syscall.Rusage
		wpid, err := syscall.Wait4(pid, &status, 0, &rusage)
		if err != nil {
			return fmt.Errorf("supervisor.Watch: wait4: %w", err)
		}

		if wpid != pid {
			continue // Not our process
		}

		if status.Exited() {
			if status.ExitStatus() == 0 && policy.Name == "on-failure" {
				return nil
			}
			if policy.Name == "always" {
				return nil
			}
			return fmt.Errorf("supervisor.Watch: process exited with status %d", status.ExitStatus())
		}

		if status.Signaled() {
			return fmt.Errorf("supervisor.Watch: process killed by signal %d", status.Signal())
		}
	}
}

// ExecInContainer runs a command in a new namespace set.
func ExecInContainer(command []string, cloneFlags uintptr) (int, error) {
	cmd := exec.Command(command[0], command[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:   cloneFlags,
		UidMappings:  []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}},
		GidMappings:  []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		return 0, fmt.Errorf("supervisor.ExecInContainer: start: %w", err)
	}

	err = cmd.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 0, fmt.Errorf("supervisor.ExecInContainer: wait: %w", err)
	}

	return 0, nil
}
