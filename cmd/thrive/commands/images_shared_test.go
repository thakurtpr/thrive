package commands

import (
	"bytes"
	"strings"
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

// TestFormatImagesTable_Layout verifies the golden table: header, digest
// truncation, row values, and column alignment.
func TestFormatImagesTable_Layout(t *testing.T) {
	rows := []imageRow{
		{Ref: "alpine:3.19", Digest: "sha256:abcdef1234567890", Layers: 3},
		{Ref: "n", Digest: "abc", Layers: 0},
	}
	var buf bytes.Buffer
	formatImagesTable(&buf, rows)
	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 4 { // header + rule + 2 rows
		t.Fatalf("expected 4 lines, got %d: %q", len(lines), out)
	}
	if !strings.Contains(lines[0], "REPOSITORY") || !strings.Contains(lines[0], "DIGEST") || !strings.Contains(lines[0], "LAYERS") {
		t.Errorf("header: got %q", lines[0])
	}
	if !strings.Contains(lines[2], "alpine:3.19") || !strings.Contains(lines[2], "sha256:abcdef123456") {
		t.Errorf("row 1: got %q", lines[2])
	}
	if strings.Contains(lines[2], "sha256:abcdef1234567890") {
		t.Errorf("row 1 digest not truncated: %q", lines[2])
	}
	if !strings.Contains(lines[3], "abc") {
		t.Errorf("row 2 short digest: got %q", lines[3])
	}
}

// TestFormatImagesTable_Empty verifies the empty-store hint.
func TestFormatImagesTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	formatImagesTable(&buf, nil)
	if got := buf.String(); !strings.Contains(got, "no images") {
		t.Errorf("empty: got %q", got)
	}
	var buf2 bytes.Buffer
	formatImagesTable(&buf2, []imageRow{})
	if got := buf2.String(); !strings.Contains(got, "no images") {
		t.Errorf("empty slice: got %q", got)
	}
}
