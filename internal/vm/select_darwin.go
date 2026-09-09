//go:build darwin

package vm

import "fmt"

func selectLauncher(vmType string) (launcher, error) {
	switch vmType {
	case "darwin-hv":
		return newDarwinLauncher(), nil
	default:
		return nil, fmt.Errorf("unsupported vm type: %s", vmType)
	}
}
