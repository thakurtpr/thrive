package commands

import (
	"testing"
)

// TestTruncateDigest verifies safe shortening at multiple widths,
// including the short/empty digests that raw slicing would panic on.
func TestTruncateDigest(t *testing.T) {
	cases := []struct {
		name   string
		digest string
		n      int
		want   string
	}{
		{"long to 12", "sha256:abcdef1234567890", 12, "sha256:abcde"},
		{"long to 19", "sha256:abcdef1234567890", 19, "sha256:abcdef123456"},
		{"exact", "sha256:abc", 10, "sha256:abc"},
		{"short kept whole", "abc", 12, "abc"},
		{"empty", "", 12, ""},
		{"empty zero width", "", 0, ""},
		{"zero width", "sha256:abc", 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := truncateDigest(tc.digest, tc.n); got != tc.want {
				t.Errorf("truncateDigest(%q, %d): got %q want %q", tc.digest, tc.n, got, tc.want)
			}
		})
	}
}

// TestShortDigest_Delegates verifies the pull-output helper matches
// 12-char truncation including short inputs.
func TestShortDigest_Delegates(t *testing.T) {
	if got := shortDigest("sha256:abcdef1234567890"); got != "sha256:abcde" {
		t.Errorf("shortDigest long: got %q", got)
	}
	if got := shortDigest("abc"); got != "abc" {
		t.Errorf("shortDigest short: got %q", got)
	}
}
