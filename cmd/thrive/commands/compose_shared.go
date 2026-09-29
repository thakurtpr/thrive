package commands

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// parseComposeServiceNames returns the sorted service names from a compose
// file's contents. The proxy can't use pkg/compose (linux-only), so it
// parses the spec it already ships to the daemon. Pure and portable.
func parseComposeServiceNames(spec string) ([]string, error) {
	var cf struct {
		Services map[string]any `yaml:"services"`
	}
	if err := yaml.Unmarshal([]byte(spec), &cf); err != nil {
		return nil, fmt.Errorf("compose: parse services: %w", err)
	}
	names := make([]string, 0, len(cf.Services))
	for name := range cf.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}
