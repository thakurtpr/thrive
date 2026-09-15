//go:build !linux

package commands_test

import (
	"testing"

	"github.com/thakurprasadrout/thrive/cmd/thrive/commands"
)

// TestNetworkCmd_SubCommands verifies network sub-commands.
func TestNetworkCmd_SubCommands(t *testing.T) {
	cmd := commands.NetworkCmd()
	if cmd.Use != "network" {
		t.Errorf("NetworkCmd: Use got %q, want network", cmd.Use)
	}
	subMap := map[string]bool{}
	for _, sub := range cmd.Commands() {
		subMap[sub.Use] = true
	}
	for _, want := range []string{
		"create [name]", "ls", "inspect [network]", "rm [network...]",
		"connect [network] [container]", "disconnect [network] [container]", "prune",
	} {
		if !subMap[want] {
			t.Errorf("NetworkCmd: missing sub-command %q (have: %v)", want, subMap)
		}
	}
}

// TestVolumeCmd_SubCommands verifies volume sub-commands.
func TestVolumeCmd_SubCommands(t *testing.T) {
	cmd := commands.VolumeCmd()
	if cmd.Use != "volume" {
		t.Errorf("VolumeCmd: Use got %q, want volume", cmd.Use)
	}
	subMap := map[string]bool{}
	for _, sub := range cmd.Commands() {
		subMap[sub.Use] = true
	}
	for _, want := range []string{
		"create [name]", "ls", "inspect [volume]", "rm [volume...]", "prune",
	} {
		if !subMap[want] {
			t.Errorf("VolumeCmd: missing sub-command %q (have: %v)", want, subMap)
		}
	}
}
