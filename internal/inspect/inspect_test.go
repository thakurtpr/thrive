package inspect

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestShapes_Golden verifies exact keys and JSON output for both shapes,
// incl. that no host path leaks into image layers.
func TestShapes_Golden(t *testing.T) {
	c := Container("abc123", "running", 100, map[string]any{"Image": "alpine"})
	for _, k := range []string{"type", "id", "status", "pid", "config"} {
		if _, ok := c[k]; !ok {
			t.Errorf("container shape: missing key %q", k)
		}
	}
	if c["type"] != "container" || c["id"] != "abc123" || c["pid"] != 100 {
		t.Errorf("container shape: got %v", c)
	}
	im := Image("alpine:3.19", "sha256:abc", []Layer{{Digest: "sha256:l1", Size: 10}})
	raw, _ := json.Marshal(im)
	out := string(raw)
	for _, want := range []string{`"type":"image"`, `"ref":"alpine:3.19"`, `"digest":"sha256:l1"`, `"size":10`} {
		if !strings.Contains(out, want) {
			t.Errorf("image JSON: missing %s in %s", want, out)
		}
	}
	if strings.Contains(out, "Path") || strings.Contains(out, "/var/lib") {
		t.Errorf("image JSON leaks host paths: %s", out)
	}
	empty, _ := json.Marshal(Image("x", "y", nil))
	if !strings.Contains(string(empty), `"layers":[]`) {
		t.Errorf("empty layers: got %s", empty)
	}
}
