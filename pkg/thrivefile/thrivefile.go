//go:build linux
// +build linux

package thrivefile

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Thrivefile struct {
	Name       string
	Base       string
	Steps      map[string]*Step
	Entrypoint []string
	Expose     []int
}

type Step struct {
	Name      string
	Run       string
	Copy      []CopySpec
	DependsOn []string
	CacheKey  string
}

type CopySpec struct {
	Source string
	Dest   string
}

type BuildGraph struct {
	Steps     map[string]*Step
	BaseImage string
	EntryCmd  []string
	Edges     map[string][]string
}

func Parse(path string) (*BuildGraph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("thrivefile.Parse: read %s: %w", path, err)
	}

	var tf struct {
		Name       string `yaml:"name"`
		Base       string `yaml:"base"`
		Entrypoint []string `yaml:"entrypoint"`
		Expose     []int `yaml:"expose"`
		Steps      map[string]struct {
			Run       string   `yaml:"run"`
			Copy      []struct{ Src string `yaml:"src"`; Dst string `yaml:"dst"` } `yaml:"copy"`
			DependsOn []string `yaml:"depends-on"`
		} `yaml:"steps"`
	}

	if err := yaml.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("thrivefile.Parse: unmarshal: %w", err)
	}

	graph := &BuildGraph{
		Steps:     make(map[string]*Step),
		BaseImage: tf.Base,
		EntryCmd:  tf.Entrypoint,
		Edges:     make(map[string][]string),
	}

	for name, s := range tf.Steps {
		step := &Step{Name: name, Run: s.Run, DependsOn: s.DependsOn}
		for _, c := range s.Copy {
			step.Copy = append(step.Copy, CopySpec{Source: c.Src, Dest: c.Dst})
		}
		graph.Steps[name] = step
		graph.Edges[name] = s.DependsOn
	}

	return graph, nil
}
