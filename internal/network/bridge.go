//go:build linux

// Package network manages container networking: bridge creation, veth pairs, NAT, and DNS.
package network

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
)

// EnsureBridge creates the thrive0 bridge and sets up NAT if it doesn't exist.
func EnsureBridge() error {
	return EnsureBridgeWith(BridgeName, BridgeCIDR)
}

// EnsureBridgeWith creates a named bridge with the given CIDR and NAT.
func EnsureBridgeWith(bridge, cidr string) error {
	subnet := cidr
	if idx := strings.Index(cidr, "/"); idx >= 0 {
		base := strings.Split(cidr[:idx], ".")
		if len(base) == 4 {
			subnet = base[0] + "." + base[1] + ".0.0/16"
		}
	}
	if bridgeExistsByName(bridge) {
		return nil
	}

	cmds := [][]string{
		{"ip", "link", "add", bridge, "type", "bridge"},
		{"ip", "addr", "add", cidr, "dev", bridge},
		{"ip", "link", "set", bridge, "up"},
	}
	for _, args := range cmds {
		if out, err := run(args...); err != nil {
			return fmt.Errorf("network.EnsureBridge: %s: %w\n%s", strings.Join(args, " "), err, out)
		}
	}

	if err := enableNATFor(bridge, subnet); err != nil {
		return fmt.Errorf("network.EnsureBridge: NAT: %w", err)
	}

	return nil
}

// DeleteBridge removes the thrive0 bridge.
func DeleteBridge() error {
	return DeleteBridgeWith(BridgeName)
}

// DeleteBridgeWith removes a named bridge.
func DeleteBridgeWith(bridge string) error {
	if !bridgeExistsByName(bridge) {
		return nil
	}
	cmds := [][]string{
		{"ip", "link", "set", bridge, "down"},
		{"ip", "link", "delete", bridge},
	}
	for _, args := range cmds {
		run(args...) // best-effort
	}
	return nil
}

func bridgeExists() bool {
	return bridgeExistsByName(BridgeName)
}

func bridgeExistsByName(bridge string) bool {
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, iface := range ifaces {
		if iface.Name == bridge {
			return true
		}
	}
	return false
}

func enableNAT() error {
	return enableNATFor(BridgeName, Subnet)
}

func enableNATFor(bridge, subnet string) error {
	if err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644); err != nil {
		return fmt.Errorf("ip_forward: %w", err)
	}

	// Idempotent: check if the rule already exists before adding
	check := []string{"iptables", "-t", "nat", "-C", "POSTROUTING",
		"-s", subnet, "!", "-o", bridge, "-j", "MASQUERADE"}
	if _, err := run(check...); err == nil {
		return nil
	}

	add := []string{"iptables", "-t", "nat", "-A", "POSTROUTING",
		"-s", subnet, "!", "-o", bridge, "-j", "MASQUERADE"}
	if out, err := run(add...); err != nil {
		return fmt.Errorf("iptables MASQUERADE: %w\n%s", err, out)
	}
	return nil
}

func run(args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
