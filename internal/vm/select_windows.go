//go:build windows

package vm

import "fmt"

func selectLauncher(vmType string) (launcher, error) {
	switch vmType {
	case "hyperv":
		return newHyperVLauncher(), nil
	case "wsl2":
		return newWSL2Launcher(), nil
	default:
		return nil, fmt.Errorf("unsupported vm type: %s", vmType)
	}
}
