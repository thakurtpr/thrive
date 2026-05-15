//go:build linux
// +build linux

package namespace

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// UserNamespaceOptions configures user namespace UID/GID mapping.
type UserNamespaceOptions struct {
	UIDMap string // e.g., "0 1000 1"
	GIDMap string // e.g., "0 1000 1"
}

// CreateUserNamespace creates a new user namespace with optional UID/GID mapping.
func CreateUserNamespace(opts UserNamespaceOptions) error {
	err := unix.Unshare(unix.CLONE_NEWUSER)
	if err != nil {
		return fmt.Errorf("namespace.CreateUserNamespace: unshare CLONE_NEWUSER: %w", err)
	}

	if opts.UIDMap != "" {
		if err := os.WriteFile("/proc/self/uid_map", []byte(opts.UIDMap), 0644); err != nil {
			return fmt.Errorf("namespace.CreateUserNamespace: write uid_map: %w", err)
		}
	}

	if opts.GIDMap != "" {
		if err := os.WriteFile("/proc/self/setgroups", []byte("deny"), 0644); err != nil {
			// Ignore error if setgroups doesn't exist
		}
		if err := os.WriteFile("/proc/self/gid_map", []byte(opts.GIDMap), 0644); err != nil {
			return fmt.Errorf("namespace.CreateUserNamespace: write gid_map: %w", err)
		}
	}

	return nil
}

// CloneFlags returns the set of clone flags needed for a container.
func CloneFlags(network, pid, mount, uts, ipc bool) uintptr {
	flags := uintptr(0)
	if network {
		flags |= uintptr(unix.CLONE_NEWNET)
	}
	if pid {
		flags |= uintptr(unix.CLONE_NEWPID)
	}
	if mount {
		flags |= uintptr(unix.CLONE_NEWNS)
	}
	if uts {
		flags |= uintptr(unix.CLONE_NEWUTS)
	}
	if ipc {
		flags |= uintptr(unix.CLONE_NEWIPC)
	}
	return flags
}
