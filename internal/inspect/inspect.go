// Package inspect shapes `thrive inspect` output maps shared by the Linux
// CLI (cmd/thrive/commands) and the VM daemon (cmd/thrived), so JSON output
// is identical on every platform. Layer entries carry digest+size only:
// host filesystem paths must never leak into inspect output.
package inspect

// Layer is one image layer descriptor for inspect output.
type Layer struct {
	Digest string
	Size   int64
}

// Container shapes container inspect info (type/id/status/pid/config).
func Container(id, status string, pid int, cfg map[string]any) map[string]any {
	return map[string]any{
		"type":   "container",
		"id":     id,
		"status": status,
		"pid":    pid,
		"config": cfg,
	}
}

// Image shapes image inspect info (type/ref/digest/layers).
func Image(ref, digest string, layers []Layer) map[string]any {
	out := make([]map[string]any, 0, len(layers))
	for _, l := range layers {
		out = append(out, map[string]any{"digest": l.Digest, "size": l.Size})
	}
	return map[string]any{
		"type": "image", "ref": ref,
		"digest": digest, "layers": out,
	}
}
