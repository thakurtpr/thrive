package commands

import "strings"

// psRow is one container row for `thrive ps` output. Shared by the Linux
// native implementation (ps.go) and the VM-daemon proxy (ps_proxy.go) so
// filtering and --format mapping behave identically on every platform.
type psRow struct {
	ID     string
	Image  string
	Status string
	PID    int
}

// filterPsRows applies `thrive ps --filter` (status=/name=/image=) and drops
// stopped containers unless all is true. Malformed filters (no "=") are
// ignored. Pure function — no filesystem or daemon access.
func filterPsRows(rows []psRow, filters []string, all bool) []psRow {
	var status, name, image string
	for _, f := range filters {
		k, v, ok := strings.Cut(f, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "status":
			status = v
		case "name":
			name = v
		case "image":
			image = v
		}
	}
	var out []psRow
	for _, r := range rows {
		if !all && r.Status == "stopped" {
			continue
		}
		if status != "" && r.Status != status {
			continue
		}
		if name != "" && !strings.Contains(r.ID, name) {
			continue
		}
		if image != "" && !strings.Contains(r.Image, image) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// psRowMap renders a row for --format Go templates.
func psRowMap(r psRow) map[string]any {
	return map[string]any{"id": r.ID, "image": r.Image, "status": r.Status, "pid": r.PID}
}
