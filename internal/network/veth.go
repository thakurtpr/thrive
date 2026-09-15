//go:build linux

package network

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// VethPair holds the names of the two veth devices for a container.
type VethPair struct {
	Host        string
	Container   string
	ContainerIP string
}

// SetupVeth creates a veth pair for the container, attaches the host end to
// thrive0, moves the container end into the container's netns, and assigns
// an IP from the 172.18.0.0/16 subnet.
func SetupVeth(containerID string, pid int) (*VethPair, error) {
	def := defaultNetwork
	def.Name = "bridge"
	return SetupVethOn(&def, containerID, pid, "eth0")
}

// SetupVethOn attaches a container to a named network on the given interface
// (eth0 for primary, eth1+ for additional attachments).
func SetupVethOn(nw *Network, containerID string, pid int, ifName string) (*VethPair, error) {
	short := containerID
	if len(short) > 8 {
		short = short[:8]
	}
	hostVeth := "veth-" + short
	if nw.Name != "bridge" {
		suffix := nw.Bridge
		if len(suffix) > 4 {
			suffix = suffix[len(suffix)-4:]
		}
		hostVeth = "v-" + short + "-" + suffix
	}

	var ip string
	var err error
	if nw.Name == "bridge" {
		ip, err = allocateIP(containerID)
	} else {
		ip, err = AllocateOn(nw, containerID)
	}
	if err != nil {
		return nil, fmt.Errorf("network.SetupVeth: allocate IP: %w", err)
	}

	cmds := [][]string{
		{"ip", "link", "add", hostVeth, "type", "veth", "peer", "name", ifName},
		{"ip", "link", "set", hostVeth, "master", nw.Bridge},
		{"ip", "link", "set", hostVeth, "up"},
		{"ip", "link", "set", ifName, "netns", strconv.Itoa(pid)},
	}
	for _, args := range cmds {
		if out, err := run(args...); err != nil {
			return nil, fmt.Errorf("network.SetupVeth: %v: %w\n%s", args, err, out)
		}
	}

	// Configure container-side networking via nsenter into the container's netns
	ns := []string{"nsenter", "--target", strconv.Itoa(pid), "--net", "--"}
	netCmds := [][]string{
		append(ns, "ip", "link", "set", "lo", "up"),
		append(ns, "ip", "link", "set", ifName, "up"),
		append(ns, "ip", "addr", "add", ip+"/16", "dev", ifName),
	}
	if ifName == "eth0" {
		netCmds = append(netCmds, append(ns, "ip", "route", "add", "default", "via", nw.Gateway))
	}
	for _, args := range netCmds {
		if out, err := run(args...); err != nil {
			return nil, fmt.Errorf("network.SetupVeth: container net: %v: %w\n%s", args, err, out)
		}
	}

	return &VethPair{
		Host:        hostVeth,
		Container:   ifName,
		ContainerIP: ip,
	}, nil
}

// TeardownVeth removes the host-side veth device (container end removed automatically).
func TeardownVeth(containerID string) {
	short := containerID
	if len(short) > 8 {
		short = short[:8]
	}
	run("ip", "link", "delete", "veth-"+short) // best-effort
	releaseIP(containerID)
}

// TeardownVethOn removes a named-network attachment's host veth and IP.
func TeardownVethOn(nw *Network, containerID, hostVeth string) {
	if hostVeth == "" {
		short := containerID
		if len(short) > 8 {
			short = short[:8]
		}
		hostVeth = "veth-" + short
	}
	run("ip", "link", "delete", hostVeth) // best-effort
	if nw != nil && nw.Name != "bridge" {
		ReleaseOn(nw, containerID)
	} else {
		releaseIP(containerID)
	}
}

// AddPortForward wires iptables DNAT rules so hostPort → containerIP:containerPort.
// Both PREROUTING (external traffic) and OUTPUT (host-local connections) chains are set up.
func AddPortForward(containerIP string, hostPort, containerPort int, protocol string) error {
	if protocol == "" {
		protocol = "tcp"
	}
	dest := fmt.Sprintf("%s:%d", containerIP, containerPort)
	dport := strconv.Itoa(hostPort)

	preroutingArgs := []string{
		"iptables", "-t", "nat", "-A", "PREROUTING",
		"-p", protocol, "--dport", dport,
		"-j", "DNAT", "--to-destination", dest,
	}
	if out, err := run(preroutingArgs...); err != nil {
		return fmt.Errorf("network.AddPortForward PREROUTING: %w\n%s", err, out)
	}

	outputArgs := []string{
		"iptables", "-t", "nat", "-A", "OUTPUT",
		"-p", protocol, "--dport", dport,
		"-j", "DNAT", "--to-destination", dest,
	}
	if out, err := run(outputArgs...); err != nil {
		run("iptables", "-t", "nat", "-D", "PREROUTING",
			"-p", protocol, "--dport", dport,
			"-j", "DNAT", "--to-destination", dest)
		return fmt.Errorf("network.AddPortForward OUTPUT: %w\n%s", err, out)
	}
	return nil
}

// RemovePortForward removes the DNAT rules for a container port mapping.
func RemovePortForward(containerIP string, hostPort, containerPort int, protocol string) {
	if protocol == "" {
		protocol = "tcp"
	}
	dest := fmt.Sprintf("%s:%d", containerIP, containerPort)
	dport := strconv.Itoa(hostPort)

	run("iptables", "-t", "nat", "-D", "PREROUTING",
		"-p", protocol, "--dport", dport,
		"-j", "DNAT", "--to-destination", dest)
	run("iptables", "-t", "nat", "-D", "OUTPUT",
		"-p", protocol, "--dport", dport,
		"-j", "DNAT", "--to-destination", dest)
}

func allocateIP(containerID string) (string, error) {
	dir := "/run/thrive/network"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	counterPath := filepath.Join(dir, "ip_counter")
	var counter int
	if data, err := os.ReadFile(counterPath); err == nil {
		counter, _ = strconv.Atoi(string(data))
	}
	counter++

	x := (counter / 254) % 254
	y := counter%254 + 2
	ip := fmt.Sprintf("172.18.%d.%d", x, y)

	os.WriteFile(counterPath, []byte(strconv.Itoa(counter)), 0644)
	os.WriteFile(filepath.Join(dir, "ip-"+containerID), []byte(ip), 0644)

	return ip, nil
}

func releaseIP(containerID string) {
	os.Remove(filepath.Join("/run/thrive/network", "ip-"+containerID))
}
