//go:build linux
// +build linux

package namespace

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// UserNamespaceOptions configures user namespace UID/GID mapping.
type UserNamespaceOptions struct {
	UIDMap string // e.g., "0 1000 1"
	GIDMap string // e.g., "0 1000 1"
}

// CreateUserNamespace creates a new user namespace with optional UID/GID mapping.
func CreateUserNamespace(opts UserNamespaceOptions) (uintptr, error) {
	fd, err := unix.Unshare(unix.CLONE_NEWUSER)
	if err != nil {
		return 0, fmt.Errorf("namespace.CreateUserNamespace: unshare CLONE_NEWUSER: %w", err)
	}

	if opts.UIDMap != "" {
		if err := unix.WriteFile("/proc/self/uid_map", []byte(opts.UIDMap), 0644); err != nil {
			unix.Close(int(fd))
			return 0, fmt.Errorf("namespace.CreateUserNamespace: write uid_map: %w", err)
		}
	}

	if opts.GIDMap != "" {
		if err := unix.WriteFile("/proc/self/setgroups", []byte("deny"), 0644); err != nil {
			// Ignore error if setgroups doesn't exist
		}
		if err := unix.WriteFile("/proc/self/gid_map", []byte(opts.GIDMap), 0644); err != nil {
			unix.Close(int(fd))
			return 0, fmt.Errorf("namespace.CreateUserNamespace: write gid_map: %w", err)
		}
	}

	return fd, nil
}

// CloneFlags returns the set of clone flags needed for a container.
func CloneFlags(network, pid, mount, uts, ipc bool) int {
	flags := 0
	if network {
		flags |= unix.CLONE_NEWNET
	}
	if pid {
		flags |= unix.CLONE_NEWPID
	}
	if mount {
		flags |= unix.CLONE_NEWNS
	}
	if uts {
		flags |= unix.CLONE_NEWUTS
	}
	if ipc {
		flags |= unix.CLONE_NEWIPC
	}
	return flags
}
