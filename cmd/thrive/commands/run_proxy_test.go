//go:build !linux

package commands

import (
	"testing"
)

// TestParseProxyPortSpecs verifies host:container[/proto] parsing for the
// run/create proxies (pure function; the daemon receives the parsed maps).
func TestParseProxyPortSpecs(t *testing.T) {
	rows := parseProxyPortSpecs([]string{"8080:80/tcp", "443:443"})
	if len(rows) != 2 {
		t.Fatalf("parseProxyPortSpecs: got %d rows, want 2", len(rows))
	}
	if rows[0]["host_port"] != 8080 || rows[0]["container_port"] != 80 || rows[0]["protocol"] != "tcp" {
		t.Errorf("row 0: got %v", rows[0])
	}
	if rows[1]["host_port"] != 443 || rows[1]["container_port"] != 443 || rows[1]["protocol"] != "tcp" {
		t.Errorf("row 1 default proto: got %v", rows[1])
	}
	// Invalid specs are skipped, never half-parsed.
	bad := parseProxyPortSpecs([]string{"nocolon", "abc:80", "80:xyz", "0:80", "80:0", "/tcp", ""})
	if len(bad) != 0 {
		t.Errorf("invalid specs: got %v, want none", bad)
	}
	if got := parseProxyPortSpecs(nil); len(got) != 0 {
		t.Errorf("nil: got %v", got)
	}
}

// TestProxyParsePort verifies strict numeric parsing (0 on any non-digit).
func TestProxyParsePort(t *testing.T) {
	cases := map[string]int{"80": 80, "0": 0, "": 0, "8a": 0, "-1": 0, " 80": 0}
	for in, want := range cases {
		if got := proxyParsePort(in); got != want {
			t.Errorf("proxyParsePort(%q): got %d want %d", in, got, want)
		}
	}
}

// TestContainerOpts_Run verifies the run proxy maps flags to bridge opts,
// sending only non-zero values so daemon defaults apply untouched.
func TestContainerOpts_Run(t *testing.T) {
	cmd := RunCmd()
	mustSet := func(name, value string) {
		t.Helper()
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("set --%s: %v", name, err)
		}
	}
	mustSet("name", "web")
	mustSet("env", "A=1")
	mustSet("publish", "8080:80/tcp")
	mustSet("memory", "512m")
	mustSet("cpus", "1.5")
	mustSet("restart", "always")
	mustSet("detach", "true")
	opts := containerOpts(cmd, true)
	if opts["name"] != "web" || opts["memory"] != "512m" || opts["restart"] != "always" {
		t.Errorf("scalar opts: got %v", opts)
	}
	if opts["detach"] != true || opts["rm"] != false || opts["tty"] != false || opts["interactive"] != false {
		t.Errorf("run bool opts: got %v", opts)
	}
	env, _ := opts["env"].([]string)
	if len(env) != 1 || env[0] != "A=1" {
		t.Errorf("env opts: got %v", opts["env"])
	}
	ports, _ := opts["ports"].([]map[string]any)
	if len(ports) != 1 || ports[0]["host_port"] != 8080 {
		t.Errorf("ports opts: got %v", opts["ports"])
	}
	// Unset optionals are absent (daemon defaults apply).
	for _, k := range []string{"network", "cpu_shares", "pids_limit", "volumes", "secrets", "configs"} {
		if _, ok := opts[k]; ok {
			t.Errorf("unset opt %q present: %v", k, opts[k])
		}
	}
}

// TestContainerOpts_Create verifies create shares the mapping minus the
// run-only booleans.
func TestContainerOpts_Create(t *testing.T) {
	cmd := CreateCmd()
	if err := cmd.Flags().Set("network", "host"); err != nil {
		t.Fatalf("set --network: %v", err)
	}
	opts := containerOpts(cmd, false)
	if opts["network"] != "host" {
		t.Errorf("network opt: got %v", opts)
	}
	for _, k := range []string{"detach", "rm", "tty", "interactive"} {
		if _, ok := opts[k]; ok {
			t.Errorf("run-only opt %q present in create: %v", k, opts[k])
		}
	}
}
