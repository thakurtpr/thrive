//go:build linux
// +build linux

package dag

import (
	"fmt"
	"sort"
)

// Graph represents a directed acyclic graph.
type Graph struct {
	nodes     map[string]bool
	edges     map[string][]string
	inDegree  map[string]int
}

// New creates a new Graph from step names and edges.
// Any node referenced as a dependency but absent from nodes is auto-registered
// so TopologicalSort reports "cycle/unreachable" instead of looping forever.
func New(nodes []string, edges map[string][]string) *Graph {
	g := &Graph{
		nodes:    make(map[string]bool),
		edges:    make(map[string][]string),
		inDegree: make(map[string]int),
	}
	for _, n := range nodes {
		g.nodes[n] = true
		g.inDegree[n] = 0
	}
	for from, toList := range edges {
		g.edges[from] = toList
		for _, to := range toList {
			if !g.nodes[to] {
				g.nodes[to] = true
				g.inDegree[to] = 0
			}
			g.inDegree[to]++
		}
	}
	return g
}

// TopologicalSort returns steps grouped by parallel execution level.
// Steps with no dependencies execute first (level 0), then their dependents, etc.
func TopologicalSort(g *Graph) ([][]string, error) {
	var result [][]string
	processed := make(map[string]bool)
	level := make(map[string]int)

	// Kahn's algorithm with level tracking
	for len(processed) < len(g.nodes) {
		var current []string
		for node := range g.nodes {
			if processed[node] {
				continue
			}
			// Check if all dependencies are processed
			allDone := true
			for _, dep := range g.edges[node] {
				if !processed[dep] {
					allDone = false
					break
				}
			}
			if allDone {
				current = append(current, node)
			}
		}

		if len(current) == 0 && len(processed) < len(g.nodes) {
			return nil, fmt.Errorf("dag: cycle detected or unreachable nodes")
		}

		sort.Strings(current)
		result = append(result, current)
		for _, node := range current {
			processed[node] = true
			level[node] = len(result) - 1
		}
	}

	return result, nil
}
