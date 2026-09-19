package commands

import (
	"testing"
)

func psTestRows() []psRow {
	return []psRow{
		{ID: "abc123", Image: "alpine:3.19", Status: "running", PID: 100},
		{ID: "def456", Image: "nginx:latest", Status: "stopped", PID: 0},
		{ID: "abc789", Image: "alpine:3.18", Status: "running", PID: 200},
	}
}

func psIDs(rows []psRow) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

func equalIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestFilterPsRows_DefaultDropsStopped verifies the default (no --all)
// hides stopped containers.
func TestFilterPsRows_DefaultDropsStopped(t *testing.T) {
	got := psIDs(filterPsRows(psTestRows(), nil, false))
	if !equalIDs(got, []string{"abc123", "abc789"}) {
		t.Errorf("default filter: got %v", got)
	}
}

// TestFilterPsRows_AllKeepsStopped verifies --all retains everything.
func TestFilterPsRows_AllKeepsStopped(t *testing.T) {
	got := psIDs(filterPsRows(psTestRows(), nil, true))
	if !equalIDs(got, []string{"abc123", "def456", "abc789"}) {
		t.Errorf("--all filter: got %v", got)
	}
}

// TestFilterPsRows_StatusNameImage verifies each filter dimension.
func TestFilterPsRows_StatusNameImage(t *testing.T) {
	cases := []struct {
		name    string
		filters []string
		all     bool
		want    []string
	}{
		{"status running", []string{"status=running"}, false, []string{"abc123", "abc789"}},
		{"status stopped needs all", []string{"status=stopped"}, true, []string{"def456"}},
		{"status stopped hidden by default", []string{"status=stopped"}, false, nil},
		{"name substring", []string{"name=abc"}, false, []string{"abc123", "abc789"}},
		{"image substring", []string{"image=alpine"}, false, []string{"abc123", "abc789"}},
		{"image no match", []string{"image=redis"}, false, nil},
		{"combined", []string{"status=running", "image=3.19"}, false, []string{"abc123"}},
		{"malformed ignored", []string{"bogus"}, false, []string{"abc123", "abc789"}},
		{"unknown key ignored", []string{"foo=bar"}, false, []string{"abc123", "abc789"}},
		{"empty value matches all", []string{"status="}, false, []string{"abc123", "abc789"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := psIDs(filterPsRows(psTestRows(), tc.filters, tc.all))
			if !equalIDs(got, tc.want) {
				t.Errorf("filters %v all=%v: got %v want %v", tc.filters, tc.all, got, tc.want)
			}
		})
	}
}

// TestFilterPsRows_EmptyInput verifies nil/empty input yields empty output.
func TestFilterPsRows_EmptyInput(t *testing.T) {
	if got := filterPsRows(nil, nil, false); len(got) != 0 {
		t.Errorf("nil input: got %v", got)
	}
	if got := filterPsRows([]psRow{}, []string{"status=running"}, true); len(got) != 0 {
		t.Errorf("empty input: got %v", got)
	}
}

// TestPsRowMap verifies the --format mapping keys.
func TestPsRowMap(t *testing.T) {
	m := psRowMap(psRow{ID: "abc123", Image: "alpine", Status: "running", PID: 100})
	if m["id"] != "abc123" || m["image"] != "alpine" || m["status"] != "running" || m["pid"] != 100 {
		t.Errorf("psRowMap: got %v", m)
	}
	if out, err := renderFormat("{{.id}} {{.status}}", m); err != nil || out != "abc123 running" {
		t.Errorf("renderFormat(psRowMap): got %q, %v", out, err)
	}
}
