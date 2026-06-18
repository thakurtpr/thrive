// Package network provides container networking primitives.
// On Linux, real bridge/veth/iptables operations are used.
// On other platforms, stub implementations return ErrNotLinux.
package network

// Networker sets up and tears down container network namespaces.
type Networker interface {
	Setup(containerID string, cfg NetConfig) error
	Teardown(containerID string) error
}

// NetConfig describes the desired network for a container.
type NetConfig struct {
	BridgeName   string
	HostIP       string
	ContainerIP  string
	PortMappings []PortMapping
}
