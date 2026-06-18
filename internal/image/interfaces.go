package image

// Mounter manages per-container rootfs overlay mounts.
type Mounter interface {
	Mount(containerID, imageRef string) (mergedDir string, err error)
	Unmount(containerID string) error
}

// Puller downloads OCI images to the local content store.
type Puller interface {
	Pull(ref string) error
	List() ([]string, error)
}
