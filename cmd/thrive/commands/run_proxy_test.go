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
