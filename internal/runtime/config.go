package runtime

import "time"

// ContainerConfig holds configuration for creating a container.
type ContainerConfig struct {
	ID            string
	Image         string
	Command       []string
	Env           []string
	Mounts        []Mount
	NetworkMode   string
	Secrets       []string
	Resources     ResourceLimits
	RestartPolicy RestartPolicy
}

// Mount represents a container mount point.
type Mount struct {
	Source      string
	Destination string
	Type        string
	Options     []string
}

// ResourceLimits specifies CPU and memory limits.
type ResourceLimits struct {
	MemoryLimit int64
	CPUQuota    int64
	CPUShares   int64
}

// RestartPolicy defines how the supervisor handles container exit.
type RestartPolicy struct {
	Name              string
	MaxRetryAttempts  int
}

// ContainerState represents the current state of a container.
type ContainerState struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	PID         int       `json:"pid"`
	StartedAt   time.Time `json:"startedAt"`
	FinishedAt  time.Time `json:"finishedAt"`
	ExitCode    int       `json:"exitCode"`
	RootfsPath  string    `json:"rootfsPath"`
	NetworkInfo string    `json:"networkInfo,omitempty"`
}

// Container represents a running or stopped container.
type Container struct {
	ID     string
	Config *ContainerConfig
	State  *ContainerState
	PID    int
}
