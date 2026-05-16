//go:build linux
// +build linux

package platform

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"
)

// Name returns the current platform name
func Name() string {
	return runtime.GOOS
}

// IsSupported returns true if the platform supports containerization
func IsSupported() bool {
	return true
}

// Reason returns an empty string since platform is supported
func Reason() string {
	return ""
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

// linuxRuntime implements Runtime for Linux
type linuxRuntime struct{}

func (r *linuxRuntime) Name() string    { return "linux" }
func (r *linuxRuntime) IsSupported() bool { return true }
func (r *linuxRuntime) Reason() string { return "" }

func (r *linuxRuntime) SupportsRootless() bool {
	return true
}

func (r *linuxRuntime) SupportsOverlay() bool {
	f, err := os.Open("/proc/filesystems")
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "overlay") {
			return true
		}
	}
	return false
}

func (r *linuxRuntime) SupportsCgroupsV2() bool {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "cgroup2")
}

func (r *linuxRuntime) DefaultStateDir() string { return "/run/thrive" }
func (r *linuxRuntime) DefaultImageDir() string { return "/var/lib/thrive/images" }
func (r *linuxRuntime) DefaultRootfsPath() string { return "" }

func (r *linuxRuntime) DetectCgroup2Mount() string {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[2] == "cgroup2" {
			return fields[1]
		}
	}
	return ""
}

func (r *linuxRuntime) RequiresFUSE() bool {
	return true
}

func (r *linuxRuntime) Validate() error {
	if !r.SupportsCgroupsV2() {
		return fmt.Errorf("cgroups v2 not available")
	}
	if !r.SupportsOverlay() {
		return fmt.Errorf("overlayfs not available")
	}
	return nil
}

// BuildRuntime returns a Linux runtime instance
func BuildRuntime() Runtime {
	return &linuxRuntime{}
}

// CheckDocker returns false on Linux - we don't need Docker
func CheckDocker() (bool, string) {
	return false, "not needed on Linux"
}