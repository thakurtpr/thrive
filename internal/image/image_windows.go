//go:build windows

package image

import (
	"context"
	"fmt"
)

// On Windows there is no host-side image store: images live inside the
// Thrive VM, so all store-backed operations must go through the control
// socket (see cmd/thrive/commands/*_windows.go). These stubs exist so the
// shared CLI helpers (e.g. verifyPulledImage in distribution_shared.go)
// compile on Windows; they are never called at runtime because the Windows
// PullCmd verifies host-side and removes via "rmi" over the bridge.
func Pull(ctx context.Context, ref string, opts PullOptions) (*Image, error) {
	return nil, fmt.Errorf("image.Pull: no host image store on Windows — pull runs inside the VM")
}

func Remove(ctx context.Context, imageRef string) error {
	return fmt.Errorf("image.Remove: no host image store on Windows — remove via the VM daemon")
}

func List(ctx context.Context) ([]*Image, error) {
	return nil, fmt.Errorf("image.List: no host image store on Windows — list via the VM daemon")
}

func Push(ctx context.Context, ref string, opts PushOptions) error {
	return fmt.Errorf("image.Push: no host image store on Windows")
}

func Mount(ctx context.Context, imageRef, containerID string) (string, error) {
	return "", fmt.Errorf("image.Mount: no host image store on Windows")
}

func Unmount(ctx context.Context, containerID string) error { return nil }
