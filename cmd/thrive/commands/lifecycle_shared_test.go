package commands

import (
	"bytes"
	"strings"
	"testing"
)

// TestParseTopProcs verifies daemon JSON decoding incl. float64 numbers,
// skipped non-maps, and missing keys.
func TestParseTopProcs(t *testing.T) {
	raw := []any{
		map[string]any{"pid": float64(10), "ppid": float64(1), "cmd": "/bin/sh"},
		map[string]any{"pid": 20, "ppid": 10, "cmd": "sleep 60"},
		"bogus",
		map[string]any{},
	}
	rows := parseTopProcs(raw)
	if len(rows) != 3 {
		t.Fatalf("parseTopProcs: got %d rows, want 3 (bogus skipped)", len(rows))
	}
	if rows[0] != (topRow{PID: 10, PPID: 1, Cmd: "/bin/sh"}) {
		t.Errorf("row 0: got %+v", rows[0])
	}
	if rows[1] != (topRow{PID: 20, PPID: 10, Cmd: "sleep 60"}) {
		t.Errorf("row 1: got %+v", rows[1])
	}
	if rows[2] != (topRow{}) {
		t.Errorf("empty map: got %+v", rows[2])
	}
	if got := parseTopProcs(nil); len(got) != 0 {
		t.Errorf("nil: got %v", got)
	}
}

// TestFormatTopTable verifies header + row rendering.
func TestFormatTopTable(t *testing.T) {
	var buf bytes.Buffer
	formatTopTable(&buf, []topRow{{PID: 10, PPID: 1, Cmd: "/bin/sh"}})
	out := buf.String()
	if !strings.Contains(out, "PID") || !strings.Contains(out, "10") || !strings.Contains(out, "/bin/sh") {
		t.Errorf("formatTopTable: got %q", out)
	}
}

// TestPortRows verifies parse + private-port filter + line format.
func TestPortRows(t *testing.T) {
	raw := []any{
		map[string]any{"container_port": float64(80), "protocol": "tcp", "host_port": float64(8080)},
		map[string]any{"container_port": float64(443), "protocol": "tcp", "host_port": float64(8443)},
		"bogus",
	}
	rows := parsePortMaps(raw)
	if len(rows) != 2 {
		t.Fatalf("parsePortMaps: got %d rows, want 2", len(rows))
	}
	if rows[0] != (portRow{ContainerPort: 80, Protocol: "tcp", HostPort: 8080}) {
		t.Errorf("row 0: got %+v", rows[0])
	}
	// Empty filter keeps all; "80" keeps one; "22" keeps none.
	if got := filterPortRows(rows, ""); len(got) != 2 {
		t.Errorf("empty filter: got %v", got)
	}
	filtered := filterPortRows(rows, "80")
	if len(filtered) != 1 || filtered[0].HostPort != 8080 {
		t.Errorf("filter 80: got %v", filtered)
	}
	if got := filterPortRows(rows, "22"); len(got) != 0 {
		t.Errorf("filter 22: got %v", got)
	}
	var buf bytes.Buffer
	formatPortLines(&buf, filtered)
	if got := buf.String(); got != "80/tcp -> 0.0.0.0:8080\n" {
		t.Errorf("formatPortLines: got %q", got)
	}
}

// TestDiffRows verifies parse + line format incl. skipped non-maps.
func TestDiffRows(t *testing.T) {
	raw := []any{
		map[string]any{"kind": "A", "path": "/new"},
		map[string]any{"kind": "D", "path": "/gone"},
		42,
	}
	rows := parseDiffChanges(raw)
	if len(rows) != 2 {
		t.Fatalf("parseDiffChanges: got %d rows, want 2", len(rows))
	}
	var buf bytes.Buffer
	formatDiffLines(&buf, rows)
	if got := buf.String(); got != "A /new\nD /gone\n" {
		t.Errorf("formatDiffLines: got %q", got)
	}
	var empty bytes.Buffer
	formatDiffLines(&empty, nil)
	if empty.String() != "" {
		t.Errorf("empty diff: got %q", empty.String())
	}
}
