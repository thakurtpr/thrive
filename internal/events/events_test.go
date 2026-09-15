package events

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testOverride(t *testing.T) {
	t.Helper()
	eventsFileOverride = filepath.Join(t.TempDir(), "events.log")
	t.Cleanup(func() { eventsFileOverride = "" })
}

// TestLogQuery_Roundtrip verifies events persist and query back.
func TestLogQuery_Roundtrip(t *testing.T) {
	testOverride(t)
	Log("container", "create", "ctr1", map[string]string{"image": "alpine"})
	Log("container", "start", "ctr1", nil)
	Log("image", "pull", "alpine", nil)

	all, err := Query(Filter{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("Query: got %d events, want 3", len(all))
	}
	if all[0].Action != "create" || all[0].Actor != "ctr1" {
		t.Errorf("first event: got %+v", all[0])
	}
	if all[0].Attributes["image"] != "alpine" {
		t.Errorf("attributes: got %v", all[0].Attributes)
	}
}

// TestQuery_TypeFilter verifies type filtering.
func TestQuery_TypeFilter(t *testing.T) {
	testOverride(t)
	Log("container", "create", "c1", nil)
	Log("image", "pull", "alpine", nil)

	got, err := Query(Filter{Type: "image"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 1 || got[0].Type != "image" {
		t.Errorf("Query type=image: got %+v", got)
	}
}

// TestQuery_TimeRange verifies since/until filtering.
func TestQuery_TimeRange(t *testing.T) {
	testOverride(t)
	Log("container", "first", "c1", nil)
	mid := time.Now().UTC()
	time.Sleep(10 * time.Millisecond)
	Log("container", "second", "c1", nil)

	got, err := Query(Filter{Since: mid})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 1 || got[0].Action != "second" {
		t.Errorf("Query since: got %+v", got)
	}

	got, err = Query(Filter{Until: mid})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 1 || got[0].Action != "first" {
		t.Errorf("Query until: got %+v", got)
	}
}

// TestQuery_MissingFile verifies empty results when no log exists.
func TestQuery_MissingFile(t *testing.T) {
	testOverride(t)
	got, err := Query(Filter{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Query: got %d events, want 0", len(got))
	}
}

// TestFormat verifies the text rendering contains key fields.
func TestFormat(t *testing.T) {
	e := Event{Time: time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC),
		Type: "container", Action: "die", Actor: "abc",
		Attributes: map[string]string{"exitCode": "0"}}
	s := Format(e)
	for _, want := range []string{"container", "die", "abc", "exitCode=0", "2026-09-15"} {
		if !strings.Contains(s, want) {
			t.Errorf("Format(%q): missing %q", s, want)
		}
	}
}
