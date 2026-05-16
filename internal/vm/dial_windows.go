//go:build windows

package vm

import (
	"context"
	"fmt"
)

// Dial returns a platform-appropriate Bridge for Windows.
// vm_type values: "hyperv", "wsl2"
func Dial(ctx context.Context, vmType string) (Bridge, error) {
	switch vmType {
	case "hyperv":
		return newHyperVBridge()
	case "wsl2":
		return newWSL2Bridge()
	default:
		return nil, fmt.Errorf("unknown vm type: %s", vmType)
	}
}