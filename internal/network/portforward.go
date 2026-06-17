//go:build linux

package network

import "fmt"

// PublishPort installs iptables DNAT rules so that connections to hostPort on
// the host are forwarded to containerPort on containerIP.
func PublishPort(hostPort, containerPort int, containerIP string) error {
	if err := validatePort(hostPort); err != nil {
		return fmt.Errorf("network.PublishPort: hostPort: %w", err)
	}
	if err := validatePort(containerPort); err != nil {
		return fmt.Errorf("network.PublishPort: containerPort: %w", err)
	}
	if containerIP == "" {
		return fmt.Errorf("network.PublishPort: containerIP must not be empty")
	}
	return AddPortForward(containerIP, hostPort, containerPort, "tcp")
}

// UnpublishPort removes the iptables DNAT rules for hostPort. Best-effort.
func UnpublishPort(hostPort, containerPort int) {
	run("iptables", "-t", "nat", "-D", "PREROUTING",
		"-p", "tcp", "--dport", fmt.Sprintf("%d", hostPort),
		"-j", "DNAT")
	run("iptables", "-t", "nat", "-D", "OUTPUT",
		"-p", "tcp", "--dport", fmt.Sprintf("%d", hostPort),
		"-j", "DNAT")
}

// PortMapping maps a host port to a container port.
// Mirrors runtime.PortMapping to avoid circular imports.
type PortMapping struct {
	HostPort      int
	ContainerPort int
	Protocol      string // "tcp" or "udp"
}

// SetupContainer wires up networking for a freshly-started container:
// ensures the bridge is up, creates a veth pair, writes resolv.conf, and
// installs port forwarding rules. Returns the assigned container IP.
func SetupContainer(containerID string, pid int, rootfsPath string, ports []PortMapping) (string, error) {
	if err := EnsureBridge(); err != nil {
		return "", fmt.Errorf("network.SetupContainer: bridge: %w", err)
	}
	veth, err := SetupVeth(containerID, pid)
	if err != nil {
		return "", fmt.Errorf("network.SetupContainer: veth: %w", err)
	}
	if rootfsPath != "" {
		WriteResolvConf(rootfsPath) //nolint:errcheck
	}
	for _, pm := range ports {
		AddPortForward(veth.ContainerIP, pm.HostPort, pm.ContainerPort, pm.Protocol) //nolint:errcheck
	}
	return veth.ContainerIP, nil
}

// TeardownContainer removes all network resources for a stopped container.
func TeardownContainer(containerID string, containerIP string, ports []PortMapping) {
	for _, pm := range ports {
		RemovePortForward(containerIP, pm.HostPort, pm.ContainerPort, pm.Protocol)
	}
	TeardownVeth(containerID)
}

func validatePort(port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("port %d out of range [1, 65535]", port)
	}
	return nil
}
