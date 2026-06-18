package runtime

// Runner is the contract for starting and stopping containers.
type Runner interface {
	Start(cfg ContainerConfig) (containerID string, err error)
	Stop(containerID string) error
	Status(containerID string) (ContainerState, error)
	Logs(containerID string) ([]byte, error)
}
