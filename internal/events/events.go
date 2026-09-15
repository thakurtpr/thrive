// Package events implements a persistent container-engine event log
// (docker events parity). Events are appended as JSON lines; readers
// support time ranges, type filters, and follow mode. Portable — the
// daemon records on Linux, tests redirect via eventsFileOverride.
package events

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Event is one engine event.
type Event struct {
	Time       time.Time         `json:"time"`
	Type       string            `json:"type"`
	Action     string            `json:"action"`
	Actor      string            `json:"actor"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// eventsFileOverride redirects the event log in tests.
var eventsFileOverride string

func eventsFile() string {
	if eventsFileOverride != "" {
		return eventsFileOverride
	}
	if runtime.GOOS == "linux" {
		return "/var/lib/thrive/events.log"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".thrive", "events.log")
}

// Log appends an event. It never fails the caller: persistence errors are
// silently dropped (observability must not break the data path).
func Log(eventType, action, actor string, attrs map[string]string) {
	e := Event{
		Time:       time.Now().UTC(),
		Type:       eventType,
		Action:     action,
		Actor:      actor,
		Attributes: attrs,
	}
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	path := eventsFile()
	os.MkdirAll(filepath.Dir(path), 0755) //nolint:errcheck
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()               //nolint:errcheck
	fmt.Fprintln(f, string(data)) //nolint:errcheck
}

// Filter constrains event queries.
type Filter struct {
	Since time.Time
	Until time.Time
	Type  string // e.g. "container", "image", "network", "volume"; empty = all
}

// Query returns events matching the filter, oldest first.
func Query(f Filter) ([]Event, error) {
	data, err := os.ReadFile(eventsFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("events: read: %w", err)
	}
	var out []Event
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		if !f.Since.IsZero() && e.Time.Before(f.Since) {
			continue
		}
		if !f.Until.IsZero() && e.Time.After(f.Until) {
			continue
		}
		if f.Type != "" && e.Type != f.Type {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// Format renders an event in docker-events-like text form.
func Format(e Event) string {
	attrs := ""
	if len(e.Attributes) > 0 {
		parts := make([]string, 0, len(e.Attributes))
		for k, v := range e.Attributes {
			parts = append(parts, k+"="+v)
		}
		attrs = " (" + strings.Join(parts, ", ") + ")"
	}
	return fmt.Sprintf("%s %s %s %s%s",
		e.Time.Format(time.RFC3339), e.Type, e.Action, e.Actor, attrs)
}

// FormatJSON renders an event as a single JSON line.
func FormatJSON(e Event) string {
	data, err := json.Marshal(e)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// Tail returns the last n lines of the log file without parsing.
func Tail(n int) ([]string, error) {
	f, err := os.Open(eventsFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("events: open: %w", err)
	}
	defer f.Close() //nolint:errcheck
	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("events: scan: %w", err)
	}
	if n > 0 && len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}
