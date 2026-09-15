//go:build !linux

package commands_test

import (
	"testing"

	"github.com/thakurprasadrout/thrive/cmd/thrive/commands"
)

// TestPhaseEProxyCmds verifies all Phase E proxy commands and sub-commands.
func TestPhaseEProxyCmds(t *testing.T) {
	swarmSubs := map[string]bool{}
	for _, sub := range commands.SwarmCmd().Commands() {
		swarmSubs[sub.Use] = true
	}
	for _, want := range []string{"init", "join [address]", "leave", "inspect", "join-token [worker|manager]"} {
		if !swarmSubs[want] {
			t.Errorf("SwarmCmd: missing %q", want)
		}
	}

	serviceSubs := map[string]bool{}
	for _, sub := range commands.ServiceCmd().Commands() {
		serviceSubs[sub.Use] = true
	}
	for _, want := range []string{
		"create [image] [command...]", "ls", "inspect [service]", "ps [service]",
		"rm [service]", "scale [service=num...]", "logs [service]",
		"update [service]", "rollback [service]",
	} {
		if !serviceSubs[want] {
			t.Errorf("ServiceCmd: missing %q", want)
		}
	}

	stackSubs := map[string]bool{}
	for _, sub := range commands.StackCmd().Commands() {
		stackSubs[sub.Use] = true
	}
	for _, want := range []string{"deploy [stack]", "ls", "ps [stack]", "services [stack]", "rm [stack]"} {
		if !stackSubs[want] {
			t.Errorf("StackCmd: missing %q", want)
		}
	}

	buildxSubs := map[string]bool{}
	for _, sub := range commands.BuildxCmd().Commands() {
		buildxSubs[sub.Use] = true
	}
	for _, want := range []string{
		"build [path]", "bake [service...]", "ls", "create [name]",
		"rm [name]", "inspect [name]", "use [name]", "du", "prune",
	} {
		if !buildxSubs[want] {
			t.Errorf("BuildxCmd: missing %q", want)
		}
	}

	ctxSubs := map[string]bool{}
	for _, sub := range commands.ContextCmd().Commands() {
		ctxSubs[sub.Use] = true
	}
	for _, want := range []string{"create [name]", "ls", "inspect [name]", "rm [name]", "use [name]"} {
		if !ctxSubs[want] {
			t.Errorf("ContextCmd: missing %q", want)
		}
	}

	pluginSubs := map[string]bool{}
	for _, sub := range commands.PluginCmd().Commands() {
		pluginSubs[sub.Use] = true
	}
	for _, want := range []string{
		"install [name] [source]", "enable [plugin]", "disable [plugin]",
		"inspect [plugin]", "ls", "rm [plugin]",
	} {
		if !pluginSubs[want] {
			t.Errorf("PluginCmd: missing %q", want)
		}
	}

	checkpointSubs := map[string]bool{}
	for _, sub := range commands.CheckpointCmd().Commands() {
		checkpointSubs[sub.Use] = true
	}
	for _, want := range []string{"create [container] [name]", "ls [container]", "rm [container] [name]"} {
		if !checkpointSubs[want] {
			t.Errorf("CheckpointCmd: missing %q", want)
		}
	}
}
