//go:build !linux

// Package network provides no-op stubs on non-Linux platforms.
// All real networking runs inside the Linux VM (vfkit/vsock).
package network

import "errors"

// ErrNotLinux is returned by stub functions on non-Linux hosts.
var ErrNotLinux = errors.New("network: operation requires Linux")

// VethPair mirrors the Linux type so callers compile on macOS.
type VethPair struct {
	Host        string
	Container   string
	ContainerIP string
}

// PortMapping mirrors the Linux type for cross-platform callers.
type PortMapping struct {
	HostPort      int
	ContainerPort int
	Protocol      string
}

// CNIPortMapping mirrors the Linux type for cross-platform callers.
type CNIPortMapping struct {
	HostPort      int
	ContainerPort int
	Protocol      string
	HostIP        string
}

func EnsureBridge() error                                                       { return ErrNotLinux }
func DeleteBridge() error                                                       { return ErrNotLinux }
func WriteResolvConf(_ string) error                                            { return ErrNotLinux }
func SetupVeth(_ string, _ int) (*VethPair, error)                              { return nil, ErrNotLinux }
func TeardownVeth(_ string)                                                     {}
func AddPortForward(_ string, _, _ int, _ string) error                         { return ErrNotLinux }
func RemovePortForward(_ string, _, _ int, _ string)                            {}
func PublishPort(_, _ int, _ string) error                                      { return ErrNotLinux }
func UnpublishPort(_, _ int)                                                    {}
func SetupContainer(_ string, _ int, _ string, _ []PortMapping) (string, error) { return "", ErrNotLinux }
func TeardownContainer(_ string, _ string, _ []PortMapping)                     {}
func CNISetup(_ string, _ string, _ []CNIPortMapping) (string, error)           { return "", ErrNotLinux }
func CNITeardown(_ string, _ string) error                                      { return ErrNotLinux }
