package commands

import (
	"fmt"
	"io"
)

// statsRow is one container's resource snapshot for `thrive stats` output.
// Shared by the Linux native implementation (lifecycle.go) and the VM-daemon
// proxy (lifecycle_proxy.go) so the table looks identical on every platform.
type statsRow struct {
	ID         string
	Status     string
	MemCurrent int64
	MemLimit   int64
	PIDs       int64
	CPUUsec    int64
}

// shortStatsID truncates container IDs to 12 characters for table display.
func shortStatsID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// formatStatsTable writes one stats table (header + one row per entry) to w.
// Pure function of rows — no runtime or daemon access, safe to unit test.
func formatStatsTable(w io.Writer, rows []statsRow) {
	fmt.Fprintf(w, "%-13s %-9s %-12s %-12s %-6s %s\n",
		"CONTAINER ID", "STATUS", "MEM USAGE", "MEM LIMIT", "PIDS", "CPU USEC")
	for _, r := range rows {
		fmt.Fprintf(w, "%-13s %-9s %-12d %-12d %-6d %d\n",
			shortStatsID(r.ID), r.Status, r.MemCurrent, r.MemLimit, r.PIDs, r.CPUUsec)
	}
}

// statsStr extracts a string field from a decoded daemon JSON result.
func statsStr(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

// statsNum extracts an integer field from a decoded daemon JSON result,
// tolerating the float64 encoding/json produces plus json.Number and int.
func statsNum(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case int32:
		return int64(v)
	default:
		return 0
	}
}
