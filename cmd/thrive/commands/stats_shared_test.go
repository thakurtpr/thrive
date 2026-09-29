package commands

import (
	"bytes"
	"strings"
	"testing"
)

// TestFormatStatsTable_Layout verifies header, ID truncation, and row values.
func TestFormatStatsTable_Layout(t *testing.T) {
	rows := []statsRow{
		{ID: "abc123def456789", Status: "running", MemCurrent: 1024, MemLimit: 2048, PIDs: 3, CPUUsec: 999},
		{ID: "short", Status: "stopped", MemCurrent: 0, MemLimit: 0, PIDs: 0, CPUUsec: 0},
	}
	var buf bytes.Buffer
	formatStatsTable(&buf, rows[:1])
	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected header + 1 row, got %d lines: %q", len(lines), out)
	}
	if !strings.Contains(lines[0], "CONTAINER ID") || !strings.Contains(lines[0], "CPU USEC") {
		t.Errorf("header: got %q", lines[0])
	}
	if !strings.Contains(lines[1], "abc123def456") || strings.Contains(lines[1], "abc123def456789") {
		t.Errorf("ID truncation: got %q", lines[1])
	}
	for _, want := range []string{"running", "1024", "2048", "999"} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("row: missing %q in %q", want, lines[1])
		}
	}
}

// TestFormatStatsTable_ShortIDKept verifies IDs under 12 chars print whole.
func TestFormatStatsTable_ShortIDKept(t *testing.T) {
	if got := shortStatsID("abc"); got != "abc" {
		t.Errorf("shortStatsID(abc): got %q", got)
	}
	if got := shortStatsID(""); got != "" {
		t.Errorf("shortStatsID(empty): got %q", got)
	}
	var buf bytes.Buffer
	formatStatsTable(&buf, []statsRow{{ID: "short", Status: "stopped"}})
	if !strings.Contains(buf.String(), "short") {
		t.Errorf("short ID missing: %q", buf.String())
	}
}

// TestFormatStatsTable_Empty verifies header-only output for zero rows.
func TestFormatStatsTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	formatStatsTable(&buf, nil)
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "CONTAINER ID") {
		t.Errorf("empty rows: got %q", buf.String())
	}
}

// TestStatsNum_Coercions verifies daemon JSON number tolerance.
func TestStatsNum_Coercions(t *testing.T) {
	m := map[string]any{
		"f64":  float64(42),
		"i":    7,
		"i64":  int64(9),
		"s":    "not-a-number",
		"nil":  nil,
		"bool": true,
	}
	if got := statsNum(m, "f64"); got != 42 {
		t.Errorf("float64: got %d", got)
	}
	if got := statsNum(m, "i"); got != 7 {
		t.Errorf("int: got %d", got)
	}
	if got := statsNum(m, "i64"); got != 9 {
		t.Errorf("int64: got %d", got)
	}
	for _, k := range []string{"s", "nil", "bool", "missing"} {
		if got := statsNum(m, k); got != 0 {
			t.Errorf("%s: got %d, want 0", k, got)
		}
	}
	if got := statsStr(m, "missing"); got != "" {
		t.Errorf("statsStr missing: got %q", got)
	}
}

// TestClearScreen_NonTerminal verifies piped/buffered output is untouched.
func TestClearScreen_NonTerminal(t *testing.T) {
	var buf bytes.Buffer
	clearScreen(&buf)
	if buf.Len() != 0 {
		t.Errorf("buffered writer: got %q, want empty", buf.String())
	}
	clearScreen(writerFunc(func(p []byte) (int, error) { return len(p), nil }))
}

// writerFunc is an io.Writer that is not an *os.File.
type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }
