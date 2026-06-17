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

const (
	BridgeName = "thrive0"
	BridgeCIDR = "172.18.0.1/16"
	Subnet     = "172.18.0.0/16"
)

// EnsureBridge creates the thrive0 bridge and sets up NAT if it doesn't exist.
func EnsureBridge() error {
	if bridgeExists() {
		return nil
	}

	cmds := [][]string{
		{"ip", "link", "add", BridgeName, "type", "bridge"},
		{"ip", "addr", "add", BridgeCIDR, "dev", BridgeName},
		{"ip", "link", "set", BridgeName, "up"},
	}
	for _, args := range cmds {
		if out, err := run(args...); err != nil {
			return fmt.Errorf("network.EnsureBridge: %s: %w\n%s", strings.Join(args, " "), err, out)
		}
	}

	if err := enableNAT(); err != nil {
		return fmt.Errorf("network.EnsureBridge: NAT: %w", err)
	}

	return nil
}

// DeleteBridge removes the thrive0 bridge.
func DeleteBridge() error {
	if !bridgeExists() {
		return nil
	}
	cmds := [][]string{
		{"ip", "link", "set", BridgeName, "down"},
		{"ip", "link", "delete", BridgeName},
	}
	for _, args := range cmds {
		run(args...) // best-effort
	}
	return nil
}

func bridgeExists() bool {
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, iface := range ifaces {
		if iface.Name == BridgeName {
			return true
		}
	}
	return false
}

func enableNAT() error {
	if err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644); err != nil {
		return fmt.Errorf("ip_forward: %w", err)
	}

	// Idempotent: check if the rule already exists before adding
	check := []string{"iptables", "-t", "nat", "-C", "POSTROUTING",
		"-s", Subnet, "!", "-o", BridgeName, "-j", "MASQUERADE"}
	if _, err := run(check...); err == nil {
		return nil
	}

	add := []string{"iptables", "-t", "nat", "-A", "POSTROUTING",
		"-s", Subnet, "!", "-o", BridgeName, "-j", "MASQUERADE"}
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
