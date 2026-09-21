package commands

// truncateDigest shortens an image digest for table display, returning the
// full string when it is already short. Unlike a raw d[:n] slice this never
// panics on short or empty digests (e.g. locally committed images).
func truncateDigest(d string, n int) string {
	if len(d) > n {
		return d[:n]
	}
	return d
}
