//go:build !linux
// +build !linux

package platform

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Name returns the current platform name
func Name() string {
	return runtime.GOOS
}

// IsSupported returns true if the platform supports containerization
func IsSupported() bool {
	return false
}

// Reason returns why the platform is not supported
func Reason() string {
	if runtime.GOOS == "darwin" {
		return "Thrive requires a Linux VM for container runtime on macOS. Run `thrive desktop init` first."
	}
	return fmt.Sprintf("Unsupported platform: %s", runtime.GOOS)
}

// Runtime is the platform-specific runtime interface
type Runtime interface {
	Name() string
	IsSupported() bool
	Reason() string
	SupportsRootless() bool
	SupportsOverlay() bool
	SupportsCgroupsV2() bool
	DefaultRootfsPath() string
	DefaultStateDir() string
	DefaultImageDir() string
	DetectCgroup2Mount() string
	RequiresFUSE() bool
	Validate() error
}

// BuildRuntime returns an unsupported runtime for non-Linux platforms
func BuildRuntime() Runtime {
	return &unsupportedRuntime{}
}

type unsupportedRuntime struct{}

func (r *unsupportedRuntime) Name() string             { return "unsupported" }
func (r *unsupportedRuntime) IsSupported() bool       { return false }
func (r *unsupportedRuntime) Reason() string          { return Reason() }
func (r *unsupportedRuntime) SupportsRootless() bool   { return false }
func (r *unsupportedRuntime) SupportsOverlay() bool   { return false }
func (r *unsupportedRuntime) SupportsCgroupsV2() bool { return false }
func (r *unsupportedRuntime) DefaultRootfsPath() string { return "" }
func (r *unsupportedRuntime) DefaultStateDir() string  { return "" }
func (r *unsupportedRuntime) DefaultImageDir() string   { return "" }
func (r *unsupportedRuntime) DetectCgroup2Mount() string { return "" }
func (r *unsupportedRuntime) RequiresFUSE() bool        { return true }
func (r *unsupportedRuntime) Validate() error {
	return fmt.Errorf("platform not supported: %s", Reason())
}

// CheckDocker checks if Docker is available
func CheckDocker() (bool, string) {
	path, err := exec.LookPath("docker")
	if err != nil {
		return false, "docker not found"
	}
	return true, path
}