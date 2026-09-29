package commands

import (
	"fmt"
	"io"
)

// truncateDigest shortens an image digest for table display, returning the
// full string when it is already short. Unlike a raw d[:n] slice this never
// panics on short or empty digests (e.g. locally committed images).
func truncateDigest(d string, n int) string {
	if len(d) > n {
		return d[:n]
	}
	return d
}

// imageRow is one image for `thrive images` output. Shared by the Linux
// native, macOS host-side, and Windows VM-daemon implementations so the
// table is identical on every platform.
type imageRow struct {
	Ref    string
	Digest string
	Layers int
}

// formatImagesTable writes the REPOSITORY/DIGEST/LAYERS table to w,
// including the empty-store hint. Digests display truncated to 19 chars.
// Pure function — golden-testable.
func formatImagesTable(w io.Writer, rows []imageRow) {
	if len(rows) == 0 {
		fmt.Fprintln(w, "no images — run `thrive pull <image>` first")
		return
	}
	fmt.Fprintf(w, "%-50s %-22s %s\n", "REPOSITORY", "DIGEST", "LAYERS")
	fmt.Fprintln(w, "────────────────────────────────────────────────────────────────────────────────")
	for _, r := range rows {
		fmt.Fprintf(w, "%-50s %-22s %d\n", r.Ref, truncateDigest(r.Digest, 19), r.Layers)
	}
}
