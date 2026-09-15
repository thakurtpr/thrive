package commands

import (
	"bytes"
	"fmt"
	"text/template"
)

// renderFormat applies a Go template to inspect data (docker --format parity).
func renderFormat(format string, data any) (string, error) {
	tmpl, err := template.New("inspect").Parse(format)
	if err != nil {
		return "", fmt.Errorf("invalid --format template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("inspect --format: %w", err)
	}
	return buf.String(), nil
}
