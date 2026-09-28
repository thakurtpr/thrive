package commands

import (
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"
)

// updateOpts reads the resource-limit flag set into the daemon bridge map.
// Portable and pure (no dial); the proxy sends all four keys always (empty
// / zero included) and the daemon ignores unset values — preserved exactly.
func updateOpts(cmd *cobra.Command) map[string]any {
	memory, _ := cmd.Flags().GetString("memory")
	cpuQuota, _ := cmd.Flags().GetInt64("cpu-quota")
	cpuShares, _ := cmd.Flags().GetInt64("cpu-shares")
	pidsLimit, _ := cmd.Flags().GetInt64("pids-limit")
	return map[string]any{
		"memory": memory, "cpu_quota": cpuQuota, "cpu_shares": cpuShares, "pids_limit": pidsLimit,
	}
}

// Shared output helpers for `thrive top` / `port` / `diff` (no build tag).
// The Linux native commands (lifecycle.go) and the VM-daemon proxies
// (lifecycle_proxy.go) render through these so tables are identical on
// every platform. All helpers are pure and unit-testable.

// topRow is one process row for `thrive top`.
type topRow struct {
	PID  int
	PPID int
	Cmd  string
}

// parseTopProcs converts a decoded daemon "processes" value into rows,
// tolerating the float64 encoding/json produces for numbers.
func parseTopProcs(v any) []topRow {
	list, _ := v.([]any)
	var out []topRow
	for _, p := range list {
		pm, ok := p.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, topRow{
			PID:  int(statsNum(pm, "pid")),
			PPID: int(statsNum(pm, "ppid")),
			Cmd:  statsStr(pm, "cmd"),
		})
	}
	return out
}

// formatTopTable writes the PID/PPID/CMD table to w.
func formatTopTable(w io.Writer, rows []topRow) {
	fmt.Fprintf(w, "%-8s %-8s %s\n", "PID", "PPID", "CMD")
	for _, r := range rows {
		fmt.Fprintf(w, "%-8d %-8d %s\n", r.PID, r.PPID, r.Cmd)
	}
}

// portRow is one mapping for `thrive port`.
type portRow struct {
	ContainerPort int
	Protocol      string
	HostPort      int
}

// parsePortMaps converts a decoded daemon "ports" value into rows.
func parsePortMaps(v any) []portRow {
	list, _ := v.([]any)
	var out []portRow
	for _, p := range list {
		pm, ok := p.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, portRow{
			ContainerPort: int(statsNum(pm, "container_port")),
			Protocol:      statsStr(pm, "protocol"),
			HostPort:      int(statsNum(pm, "host_port")),
		})
	}
	return out
}

// filterPortRows keeps only rows matching the private-port filter.
// An empty filter keeps everything (docker `port` parity with Linux).
func filterPortRows(rows []portRow, filter string) []portRow {
	if filter == "" {
		return rows
	}
	var out []portRow
	for _, r := range rows {
		if strconv.Itoa(r.ContainerPort) == filter {
			out = append(out, r)
		}
	}
	return out
}

// formatPortLines writes one "CONTAINER/proto -> 0.0.0.0:HOST" line per row.
func formatPortLines(w io.Writer, rows []portRow) {
	for _, r := range rows {
		fmt.Fprintf(w, "%d/%s -> 0.0.0.0:%d\n", r.ContainerPort, r.Protocol, r.HostPort)
	}
}

// diffRow is one filesystem change for `thrive diff`.
type diffRow struct {
	Kind string
	Path string
}

// parseDiffChanges converts a decoded daemon "changes" value into rows.
func parseDiffChanges(v any) []diffRow {
	list, _ := v.([]any)
	var out []diffRow
	for _, c := range list {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, diffRow{Kind: statsStr(cm, "kind"), Path: statsStr(cm, "path")})
	}
	return out
}

// formatDiffLines writes one "KIND /path" line per row.
func formatDiffLines(w io.Writer, rows []diffRow) {
	for _, r := range rows {
		fmt.Fprintf(w, "%s %s\n", r.Kind, r.Path)
	}
}
