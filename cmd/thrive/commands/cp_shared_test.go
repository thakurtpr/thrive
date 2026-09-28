package commands

import (
	"testing"
)

// TestParseCpArgs verifies direction detection shared by Linux and proxy:
// colon in SRC copies from the container, else colon in DST copies to it.
func TestParseCpArgs(t *testing.T) {
	cases := []struct {
		name             string
		src, dst         string
		id, spath, dpath string
		toContainer      bool
	}{
		{"from container", "ctr:/a/b", "/local", "ctr", "/a/b", "/local", false},
		{"to container", "/local", "ctr:/a/b", "ctr", "/local", "/a/b", true},
		{"src wins", "c1:/a", "c2:/b", "c1", "/a", "c2:/b", false},
		{"no colon", "/a", "/b", "", "/a", "/b", false},
		{"leading colon ignored", ":/a", "/b", "", ":/a", "/b", false},
		{"dst leading colon ignored", "/a", ":/b", "", "/a", ":/b", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, sp, dp, toC := parseCpArgs(tc.src, tc.dst)
			if id != tc.id || sp != tc.spath || dp != tc.dpath || toC != tc.toContainer {
				t.Errorf("parseCpArgs(%q, %q): got %q %q %q %v", tc.src, tc.dst, id, sp, dp, toC)
			}
		})
	}
}
