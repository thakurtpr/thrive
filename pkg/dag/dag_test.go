//go:build linux
// +build linux

package dag

import (
	"testing"
)

func TestTopologicalSort(t *testing.T) {
	// Test case: A -> B -> C (linear chain)
	g := New([]string{"a", "b", "c"}, map[string][]string{
		"a": {},
		"b": {"a"},
		"c": {"b"},
	})

	levels, err := TopologicalSort(g)
	if err != nil {
		t.Fatalf("TopologicalSort failed: %v", err)
	}

	if len(levels) != 3 {
		t.Errorf("Expected 3 levels, got %d", len(levels))
	}

	// Level 0 should be "a"
	if len(levels[0]) != 1 || levels[0][0] != "a" {
		t.Errorf("Level 0 expected [a], got %v", levels[0])
	}

	// Level 1 should be "b"
	if len(levels[1]) != 1 || levels[1][0] != "b" {
		t.Errorf("Level 1 expected [b], got %v", levels[1])
	}

	// Level 2 should be "c"
	if len(levels[2]) != 1 || levels[2][0] != "c" {
		t.Errorf("Level 2 expected [c], got %v", levels[2])
	}
}

func TestTopologicalSortParallel(t *testing.T) {
	// Test case: A, B, C all independent
	g := New([]string{"a", "b", "c"}, map[string][]string{
		"a": {},
		"b": {},
		"c": {},
	})

	levels, err := TopologicalSort(g)
	if err != nil {
		t.Fatalf("TopologicalSort failed: %v", err)
	}

	if len(levels) != 1 {
		t.Errorf("Expected 1 level (all parallel), got %d", len(levels))
	}

	if len(levels[0]) != 3 {
		t.Errorf("Expected 3 nodes in level 0, got %d", len(levels[0]))
	}
}

func TestTopologicalSortDiamond(t *testing.T) {
	// Test case: A -> B, A -> C, B -> D, C -> D (diamond)
	g := New([]string{"a", "b", "c", "d"}, map[string][]string{
		"a": {},
		"b": {"a"},
		"c": {"a"},
		"d": {"b", "c"},
	})

	levels, err := TopologicalSort(g)
	if err != nil {
		t.Fatalf("TopologicalSort failed: %v", err)
	}

	if len(levels) != 3 {
		t.Errorf("Expected 3 levels (a, then b/c, then d), got %d", len(levels))
	}

	// Level 0 should be "a" only
	if len(levels[0]) != 1 || levels[0][0] != "a" {
		t.Errorf("Level 0 expected [a], got %v", levels[0])
	}

	// Level 1 should have b and c
	if len(levels[1]) != 2 {
		t.Errorf("Level 1 expected 2 nodes, got %d", len(levels[1]))
	}

	// Level 2 should be "d"
	if len(levels[2]) != 1 || levels[2][0] != "d" {
		t.Errorf("Level 2 expected [d], got %v", levels[2])
	}
}

func TestTopologicalSortNoNodes(t *testing.T) {
	g := New([]string{}, map[string][]string{})
	levels, err := TopologicalSort(g)
	if err != nil {
		t.Fatalf("TopologicalSort failed: %v", err)
	}
	if len(levels) != 0 {
		t.Errorf("Expected 0 levels for empty graph, got %d", len(levels))
	}
}
