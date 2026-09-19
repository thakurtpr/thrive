package registry

import (
	"context"
	"testing"
)

// TestListRepoTags_InvalidRef verifies bad references fail before any network.
func TestListRepoTags_InvalidRef(t *testing.T) {
	if _, err := ListRepoTags(context.Background(), ":::not-a-ref:::", "", ""); err == nil {
		t.Error("ListRepoTags: expected error for invalid ref, got nil")
	}
}

// TestListRepoTags_EmptyRef verifies empty references fail fast.
func TestListRepoTags_EmptyRef(t *testing.T) {
	if _, err := ListRepoTags(context.Background(), "", "", ""); err == nil {
		t.Error("ListRepoTags: expected error for empty ref, got nil")
	}
}
