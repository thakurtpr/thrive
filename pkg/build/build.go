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
	"time"

	"github.com/thakurprasadrout/thrive/internal/runtime"
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

	// Execute each level in parallel; levels are already topologically ordered.
	stepOutputs := make(map[string]string)
	for levelIdx, level := range levels {
		fmt.Printf("build: level %d — steps: %v\n", levelIdx, level)

		eg, egCtx := errgroup.WithContext(ctx)
		for _, stepName := range level {
			stepName := stepName // capture for goroutine
			step := graph.Steps[stepName]
			eg.Go(func() error {
				cacheKey, err := CacheKey(step, graph.BaseImage, stepOutputs)
				if err != nil {
					return fmt.Errorf("build.Execute: CacheKey for %s: %w", stepName, err)
				}
				step.CacheKey = cacheKey

				cachePath := fmt.Sprintf("/var/lib/thrive/cache/%s", cacheKey)
				if !opts.NoCache {
					if _, err := os.Stat(cachePath); err == nil {
						fmt.Printf("build: step %s — cache hit (%s)\n", stepName, cacheKey[:12])
						stepOutputs[stepName] = cachePath
						return nil
					}
				}

				// Run step inside a container using the build base image.
				fmt.Printf("build: step %s — executing: %s\n", stepName, step.Run)
				containerID := fmt.Sprintf("thrive-build-%s-%s", stepName, cacheKey[:8])
				cfg := runtime.ContainerConfig{
					ID:      containerID,
					Image:   graph.BaseImage,
					Command: []string{"/bin/sh", "-c", step.Run},
				}

				if _, err := runtime.Create(egCtx, cfg); err != nil {
					return fmt.Errorf("build.Execute: Create %s: %w", stepName, err)
				}
				if err := runtime.Start(egCtx, containerID); err != nil {
					runtime.Delete(egCtx, containerID)
					return fmt.Errorf("build.Execute: Start %s: %w", stepName, err)
				}

				// Poll until the step container stops.
				for {
					state, err := runtime.State(egCtx, containerID)
					if err != nil {
						break
					}
					if state.Status == "stopped" {
						runtime.Delete(egCtx, containerID)
						if state.ExitCode != 0 {
							return fmt.Errorf("build.Execute: step %s failed (exit %d)", stepName, state.ExitCode)
						}
						break
					}
					time.Sleep(100 * time.Millisecond)
				}

				// Mark cache hit for downstream steps.
				os.MkdirAll("/var/lib/thrive/cache", 0755)
				os.WriteFile(cachePath, []byte("done"), 0644)
				stepOutputs[stepName] = cachePath
				fmt.Printf("build: step %s — done\n", stepName)
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
