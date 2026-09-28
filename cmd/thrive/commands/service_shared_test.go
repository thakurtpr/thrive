package commands

import (
	"testing"

	"github.com/spf13/cobra"
)

// findSub locates a subcommand by Use for opts-mapping tests.
func findSub(t *testing.T, root *cobra.Command, use string) *cobra.Command {
	t.Helper()
	for _, sub := range root.Commands() {
		if sub.Use == use {
			return sub
		}
	}
	t.Fatalf("missing %q subcommand", use)
	return nil
}

// TestServiceCreateOpts verifies the service-create flag→wire-map mapping,
// incl. preserved defaults (replicas 1, parallelism 1). Portable: the Linux
// and proxy create subcommands share Use and flag names.
func TestServiceCreateOpts(t *testing.T) {
	create := findSub(t, ServiceCmd(), "create [image] [command...]")
	for name, value := range map[string]string{
		"name": "api", "env": "A=1", "publish": "80:80",
		"network": "mynet", "restart": "always",
	} {
		if err := create.Flags().Set(name, value); err != nil {
			t.Fatalf("set --%s: %v", name, err)
		}
	}
	opts := serviceCreateOpts(create)
	if opts["name"] != "api" || opts["network"] != "mynet" || opts["restart"] != "always" {
		t.Errorf("scalar opts: got %v", opts)
	}
	if opts["replicas"] != 1 || opts["parallelism"] != 1 || opts["delay"] != 0 {
		t.Errorf("default opts: got %v", opts)
	}
	env, _ := opts["env"].([]string)
	if len(env) != 1 || env[0] != "A=1" {
		t.Errorf("env opts: got %v", opts["env"])
	}
}
