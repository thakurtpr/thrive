package image_test

import (
	"fmt"
	"testing"
)

func TestFakeStorePull(t *testing.T) {
	store := newFakeStore()
	if err := store.Pull("alpine:3.19"); err != nil {
		t.Fatalf("Pull: %v", err)
	}
	imgs, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(imgs) != 1 || imgs[0] != "alpine:3.19" {
		t.Errorf("expected [alpine:3.19], got %v", imgs)
	}
}

func TestFakeStorePullError(t *testing.T) {
	store := newFakeStore()
	store.pullErr = fmt.Errorf("registry unavailable")
	if err := store.Pull("alpine:3.19"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFakeStoreMountSuccess(t *testing.T) {
	store := newFakeStore()
	_ = store.Pull("alpine:3.19")
	dir, err := store.Mount("ctr-1", "alpine:3.19")
	if err != nil {
		t.Fatalf("Mount: %v", err)
	}
	if dir == "" {
		t.Error("expected non-empty merged dir")
	}
	if err := store.Unmount("ctr-1"); err != nil {
		t.Fatalf("Unmount: %v", err)
	}
}

func TestFakeStoreMountMissingImage(t *testing.T) {
	store := newFakeStore()
	if _, err := store.Mount("ctr-1", "nonexistent:latest"); err == nil {
		t.Fatal("expected error for missing image")
	}
}
