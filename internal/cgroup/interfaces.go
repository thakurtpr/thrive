package cgroup

// Limits describes resource constraints for a single container.
type Limits struct {
	MemoryBytes int64 // 0 = unlimited
	CPUQuota    int64 // microseconds per 100ms; 0 = unlimited
	PIDsMax     int64 // 0 = unlimited
}

// Limiter applies and removes cgroup v2 resource limits for containers.
type Limiter interface {
	Apply(containerID string, limits Limits) error
	Remove(containerID string) error
}
