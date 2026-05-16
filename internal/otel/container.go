//go:build linux
// +build linux

package otel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/thakurprasadrout/thrive/internal/telemetry"
)

type ContainerMetrics struct {
	PID              int
	CPUUsageNanos    int64
	MemoryUsage      int64
	MemoryLimit      int64
	BlockReadBytes   int64
	BlockWriteBytes  int64
}

func ReadContainerMetrics(containerID string) (*ContainerMetrics, error) {
	log := telemetry.Logger()
	log.Debug("containerMetrics.ReadContainerMetrics: starting", telemetry.FieldString("containerID", containerID))

	cgroupPath := filepath.Join("/sys/fs/cgroup/thrive.slice", "thrive-"+containerID)

	cpuUsage, err := readCgroupFile(cgroupPath, "cpu.stat")
	if err != nil {
		log.Warn("containerMetrics.ReadContainerMetrics: cpu.stat read failed", telemetry.FieldString("path", cgroupPath), telemetry.FieldError(err))
	}

	memCurrent, _ := readCgroupUint64(cgroupPath, "memory.current")
	memLimit, _ := readCgroupUint64(cgroupPath, "memory.max")

	metrics := &ContainerMetrics{
		CPUUsageNanos:  cpuUsage,
		MemoryUsage:    int64(memCurrent),
		MemoryLimit:    int64(memLimit),
	}

	log.Debug("containerMetrics.ReadContainerMetrics: done",
		telemetry.FieldString("containerID", containerID),
		telemetry.FieldInt64("cpuNanos", metrics.CPUUsageNanos),
		telemetry.FieldInt64("memUsage", metrics.MemoryUsage),
	)

	return metrics, nil
}

func readCgroupFile(base, file string) (int64, error) {
	data, err := os.ReadFile(filepath.Join(base, file))
	if err != nil {
		return 0, err
	}

	for _, line := range splitLines(string(data)) {
		if len(line) > 0 && line[0] == 'c' {
			parts := splitWords(line)
			if len(parts) >= 2 && parts[0] == "usage_usec" {
				v, _ := strconv.ParseInt(parts[1], 10, 64)
				return v * 1000, nil
			}
		}
	}
	return 0, fmt.Errorf("usage_usec not found")
}

func readCgroupUint64(base, file string) (uint64, error) {
	data, err := os.ReadFile(filepath.Join(base, file))
	if err != nil {
		return 0, err
	}
	v, _ := strconv.ParseUint(string(data[:len(data)-1]), 10, 64)
	return v, nil
}

func splitLines(s string) []string {
	result := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

func splitWords(s string) []string {
	result := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\n' {
			if start < i {
				result = append(result, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

func CollectContainerMetrics(ctx context.Context, containerID string) (*ContainerMetrics, error) {
	return ReadContainerMetrics(containerID)
}