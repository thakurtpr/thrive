//go:build !linux

package vm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	MemoryMB int    `json:"memory_mb"`
	CPUCount int    `json:"cpu_count"`
	VMType   string `json:"vm_type"`
}

type VMState struct {
	Version     string `json:"version"`
	Running     bool   `json:"running"`
	PID         int    `json:"pid"`
	CID         int    `json:"cid"`
	VMType      string `json:"vm_type"`
	WSLInstance string `json:"wsl_instance,omitempty"`
	LastStart   string `json:"last_start,omitempty"`
}

// ConfigPath returns the path to config.json
func ConfigPath() string {
	return filepath.Join(ThriveDir(), "config.json")
}

// VMStatePath returns the path to vm.json
func VMStatePath() string {
	return filepath.Join(ThriveDir(), "vm", "vm.json")
}

// ThriveDir returns the platform-specific Thrive data directory
func ThriveDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "Thrive")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".thrive")
}

// ReadConfig reads and parses config.json
func ReadConfig() (*Config, error) {
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// WriteConfig writes config.json
func WriteConfig(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(), data, 0644)
}

// ReadVMState reads and parses vm.json
func ReadVMState() (*VMState, error) {
	data, err := os.ReadFile(VMStatePath())
	if err != nil {
		return nil, err
	}
	var state VMState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// WriteVMState writes vm.json
func WriteVMState(state *VMState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	vmDir := filepath.Dir(VMStatePath())
	if err := os.MkdirAll(vmDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(VMStatePath(), data, 0644)
}

// DetectVMType returns the appropriate vm_type for the current platform
func DetectVMType() string {
	switch runtime.GOOS {
	case "darwin":
		return "darwin-hv"
	case "windows":
		if isHyperVAvailable() {
			return "hyperv"
		}
		return "wsl2"
	default:
		return ""
	}
}

func isHyperVAvailable() bool {
	// On Windows, check if Hyper-V is available via registry or WMIC
	// Default to wsl2 until proper detection is implemented
	return false
}