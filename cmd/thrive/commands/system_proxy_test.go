//go:build !linux

package commands_test

import (
	"testing"

	"github.com/thakurprasadrout/thrive/cmd/thrive/commands"
)

// TestSystemCmd_ProxySubCommands verifies df/events/prune sub-commands exist.
func TestSystemCmd_ProxySubCommands(t *testing.T) {
	cmd := commands.SystemCmd()
	subMap := map[string]bool{}
	for _, sub := range cmd.Commands() {
		subMap[sub.Use] = true
	}
	for _, want := range []string{"df", "events", "prune"} {
		if !subMap[want] {
			t.Errorf("SystemCmd: missing sub-command %q (have: %v)", want, subMap)
		}
	}
}

// TestSystemPrune_Flags verifies prune/events flags.
func TestSystemPrune_Flags(t *testing.T) {
	for _, sub := range commands.SystemCmd().Commands() {
		if sub.Use == "prune" {
			for _, flagName := range []string{"all", "volumes", "force"} {
				if f := sub.Flags().Lookup(flagName); f == nil {
					t.Errorf("System prune: missing --%s flag", flagName)
				}
			}
		}
		if sub.Use == "events" {
			for _, flagName := range []string{"since", "until", "filter", "follow"} {
				if f := sub.Flags().Lookup(flagName); f == nil {
					t.Errorf("System events: missing --%s flag", flagName)
				}
			}
		}
	}
}

// TestLogsCmd_TailFlag verifies logs exposes --tail.
func TestLogsCmd_TailFlag(t *testing.T) {
	if f := commands.LogsCmd().Flags().Lookup("tail"); f == nil {
		t.Error("LogsCmd: missing --tail flag")
	}
}

// TestInspectCmd_FormatFlag verifies inspect exposes --format.
func TestInspectCmd_FormatFlag(t *testing.T) {
	if f := commands.InspectCmd().Flags().Lookup("format"); f == nil {
		t.Error("InspectCmd: missing --format flag")
	}
}

// TestCpCmd_Use verifies cp command exists on non-linux.
func TestCpCmd_UseProxy(t *testing.T) {
	if cmd := commands.CpCmd(); cmd.Use == "" {
		t.Error("CpCmd: Use field is empty")
	}
}
