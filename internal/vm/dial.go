//go:build darwin

package vm

import (
	"context"
	"fmt"
)

// Dial returns a platform-appropriate Bridge for macOS.
// vm_type values: "darwin-hv"
func Dial(ctx context.Context, vmType string) (Bridge, error) {
	switch vmType {
	case "darwin-hv":
		return newVSOCKBridge()
	default:
		return nil, fmt.Errorf("unknown vm type: %s", vmType)
	}
}