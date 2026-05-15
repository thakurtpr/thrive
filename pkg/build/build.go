//go:build linux
// +build linux

package build

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/thakurprasadrout/thrive/pkg/dag"
	"github.com/thakurprasadrout/thrive/pkg/thrivefile"
	"golang.org/x/sync/errgroup"
)

type BuildOptions struct {
	Tag       string
	NoCache   bool
	ContextDir string
}

type BuildResult struct {
	ImageID string
	Steps   int
}

func ParseThrivefile(path string) (*thrivefile.BuildGraph, error) {
	return thrivefile.Parse(path)
}

func Execute(ctx context.Context, graph *thrivefile.BuildGraph, opts BuildOptions) (*BuildResult, error) {
	if graph == nil || len(graph.Steps) == 0 {
		return nil, fmt.Errorf("build.Execute: no steps to execute")
	}

	// Build node list and edges for DAG
	nodes := make([]string, 0, len(graph.Steps))
	for name := range graph.Steps {
		nodes = append(nodes, name)
	}

	g := dag.New(nodes, graph.Edges)
	levels, err := dag.TopologicalSort(g)
	if err != nil {
		return nil, fmt.Errorf("build.Execute: TopologicalSort: %w", err)
	}

	// Execute each level in parallel
	stepOutputs := make(map[string]string)
	for levelIdx, level := range levels {
		fmt.Printf("Executing level %d with %d steps: %v\n", levelIdx, len(level), level)

		eg, _ := errgroup.WithContext(ctx)
		for _, stepName := range level {
			step := graph.Steps[stepName]
			eg.Go(func() error {
				cacheKey, err := CacheKey(step, graph.BaseImage, stepOutputs)
				if err != nil {
					return fmt.Errorf("build.Execute: CacheKey: %w", err)
				}
				step.CacheKey = cacheKey

				// Check cache
				if !opts.NoCache {
					cachePath := fmt.Sprintf("/var/lib/thrive/cache/%s", cacheKey)
					if _, err := os.Stat(cachePath); err == nil {
						fmt.Printf("Step %s: cache hit (%s)\n", stepName, cacheKey[:12])
						stepOutputs[stepName] = cachePath
						return nil
					}
				}

				// Execute step (simplified - runs in build container)
				fmt.Printf("Step %s: executing...\n", stepName)
				stepOutputs[stepName] = "/var/lib/thrive/cache/" + cacheKey
				return nil
			})
		}
		if err := eg.Wait(); err != nil {
			return nil, err
		}
	}

	return &BuildResult{
		ImageID: opts.Tag,
		Steps:  len(graph.Steps),
	}, nil
}

func CacheKey(step *thrivefile.Step, baseImage string, inputs map[string]string) (string, error) {
	data := fmt.Sprintf("%s:%s", baseImage, step.Run)
	for _, dep := range step.DependsOn {
		if input, ok := inputs[dep]; ok {
			data += ":" + input
		}
	}
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:]), nil
}

func copyFile(src, dst string) error {
	srcF, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcF.Close()

	dstF, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstF.Close()

	_, err = io.Copy(dstF, srcF)
	return err
}
