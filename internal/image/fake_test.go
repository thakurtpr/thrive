package image_test

import (
	"fmt"
	"sync"
)

type fakeStore struct {
	mu       sync.Mutex
	images   map[string]bool
	mounts   map[string]string
	pullErr  error
	mountErr error
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		images: make(map[string]bool),
		mounts: make(map[string]string),
	}
}

func (f *fakeStore) Pull(ref string) error {
	if f.pullErr != nil {
		return f.pullErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.images[ref] = true
	return nil
}

func (f *fakeStore) List() ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.images))
	for ref := range f.images {
		out = append(out, ref)
	}
	return out, nil
}

func (f *fakeStore) Mount(containerID, imageRef string) (string, error) {
	if f.mountErr != nil {
		return "", f.mountErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.images[imageRef] {
		return "", fmt.Errorf("image not found: %s", imageRef)
	}
	dir := "/tmp/thrive-fake/" + containerID
	f.mounts[containerID] = dir
	return dir, nil
}

func (f *fakeStore) Unmount(containerID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.mounts, containerID)
	return nil
}
