//go:build linux
// +build linux

package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/thakurprasadrout/thrive/internal/cgroup"
	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/network"
	"github.com/thakurprasadrout/thrive/internal/secrets"
	"github.com/thakurprasadrout/thrive/internal/telemetry"
)

// Create initializes a new container from the given config but does not start it.
func Create(ctx context.Context, cfg ContainerConfig) (*Container, error) {
	log := telemetry.Logger()
	log.Info("runtime.Create: starting",
		telemetry.FieldString("containerID", cfg.ID),
		telemetry.FieldString("image", cfg.Image))

	if cfg.ID == "" {
		log.Error("runtime.Create: container ID required", telemetry.FieldString("error", "empty ID"))
		return nil, fmt.Errorf("runtime.Create: container ID required")
	}
	telemetry.Debug("runtime.Create: container ID validated", telemetry.FieldString("containerID", cfg.ID))

	containerDir := filepath.Join("/run/thrive/containers", cfg.ID)
	log.Info("runtime.Create: creating container directory", telemetry.FieldString("path", containerDir))

	if err := os.MkdirAll(containerDir, 0755); err != nil {
		log.Error("runtime.Create: mkdir failed", telemetry.FieldString("path", containerDir), telemetry.FieldError(err))
		return nil, fmt.Errorf("runtime.Create: mkdir %s: %w", containerDir, err)
	}
	telemetry.Debug("runtime.Create: container directory created", telemetry.FieldString("path", containerDir))

	// Save config for Start to use
	configPath := filepath.Join(containerDir, "config.json")
	log.Info("runtime.Create: marshaling config", telemetry.FieldString("configPath", configPath))

	data, err := json.Marshal(cfg)
	if err != nil {
		log.Error("runtime.Create: marshal config failed", telemetry.FieldError(err))
		return nil, fmt.Errorf("runtime.Create: marshal config: %w", err)
	}
	telemetry.Debug("runtime.Create: config marshaled", telemetry.FieldInt("size", len(data)))

	log.Info("runtime.Create: writing config file", telemetry.FieldString("configPath", configPath))
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		log.Error("runtime.Create: write config failed", telemetry.FieldString("configPath", configPath), telemetry.FieldError(err))
		return nil, fmt.Errorf("runtime.Create: write config: %w", err)
	}
	telemetry.Debug("runtime.Create: config file written", telemetry.FieldString("configPath", configPath))

	container := &Container{
		ID:     cfg.ID,
		Config: &cfg,
		State: &ContainerState{
			ID:     cfg.ID,
			Status: "created",
		},
	}
	telemetry.Debug("runtime.Create: container struct created", telemetry.FieldString("containerID", container.ID))

	log.Info("runtime.Create: saving initial state", telemetry.FieldString("containerID", cfg.ID))
	if err := saveState(containerDir, container.State); err != nil {
		log.Error("runtime.Create: saveState failed", telemetry.FieldString("containerID", cfg.ID), telemetry.FieldError(err))
		return nil, fmt.Errorf("runtime.Create: saveState: %w", err)
	}
	telemetry.Debug("runtime.Create: state saved", telemetry.FieldString("containerID", cfg.ID))

	log.Info("runtime.Create: completed successfully", telemetry.FieldString("containerID", cfg.ID))
	return container, nil
}

// Start executes the container's main process with namespace isolation.
func Start(ctx context.Context, id string) error {
	log := telemetry.Logger()
	log.Info("runtime.Start: starting", telemetry.FieldString("containerID", id))

	state, err := loadState(id)
	if err != nil {
		log.Error("runtime.Start: loadState failed", telemetry.FieldString("containerID", id), telemetry.FieldError(err))
		return fmt.Errorf("runtime.Start: %w", err)
	}
	telemetry.Debug("runtime.Start: state loaded", telemetry.FieldString("containerID", id), telemetry.FieldString("status", state.Status))

	if state.Status != "created" {
		log.Warn("runtime.Start: container not in created state",
			telemetry.FieldString("containerID", id),
			telemetry.FieldString("status", state.Status))
		return fmt.Errorf("runtime.Start: container already started or deleted")
	}
	telemetry.Debug("runtime.Start: status check passed", telemetry.FieldString("containerID", id))

	// Get container config
	configPath := filepath.Join("/run/thrive/containers", id, "config.json")
	log.Info("runtime.Start: reading config file", telemetry.FieldString("configPath", configPath))

	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Error("runtime.Start: read config failed", telemetry.FieldString("configPath", configPath), telemetry.FieldError(err))
		return fmt.Errorf("runtime.Start: read config: %w", err)
	}
	telemetry.Debug("runtime.Start: config file read", telemetry.FieldInt("size", len(data)))

	cfg := &ContainerConfig{}
	if err := json.Unmarshal(data, cfg); err != nil {
		log.Error("runtime.Start: unmarshal config failed", telemetry.FieldError(err))
		return fmt.Errorf("runtime.Start: unmarshal config: %w", err)
	}
	telemetry.Debug("runtime.Start: config unmarshaled", telemetry.FieldString("image", cfg.Image))

	// Mount image rootfs via OverlayFS (fuse-overlayfs fallback for rootless).
	log.Info("runtime.Start: mounting rootfs", telemetry.FieldString("image", cfg.Image))
	rootfsPath, mountErr := image.Mount(ctx, cfg.Image, id)
	if mountErr != nil {
		log.Warn("runtime.Start: rootfs mount failed, running without chroot",
			telemetry.FieldString("containerID", id), telemetry.FieldError(mountErr))
		rootfsPath = ""
	} else {
		log.Info("runtime.Start: rootfs mounted", telemetry.FieldString("rootfs", rootfsPath))
	}

	// Build command — default to /bin/sh if no command given
	cmd := cfg.Command
	if len(cmd) == 0 {
		cmd = []string{"/bin/sh"}
	}

	// Resolve non-absolute binary names within the rootfs. exec.Command("nginx")
	// would look in the PARENT process's PATH (which is empty for thrived's scratch
	// container). We search the rootfs so the full absolute path is used.
	binary := cmd[0]
	if rootfsPath != "" && len(binary) > 0 && binary[0] != '/' {
		found := false
		for _, dir := range []string{"/usr/sbin", "/usr/bin", "/sbin", "/bin", "/usr/local/sbin", "/usr/local/bin"} {
			if _, statErr := os.Stat(filepath.Join(rootfsPath, dir, binary)); statErr == nil {
				binary = filepath.Join(dir, binary)
				found = true
				break
			}
		}
		if !found {
			// Wrap in shell so the container's PATH is used for lookup
			shellCmd := strings.Join(cmd, " ")
			binary = "/bin/sh"
			cmd = []string{"/bin/sh", "-c", shellCmd}
		} else {
			cmd[0] = binary
		}
	}
	log.Info("runtime.Start: preparing exec.Command", telemetry.FieldString("command", binary))
	execCmd := exec.Command(binary, cmd[1:]...)
	execCmd.Args = cmd
	execCmd.Env = cfg.Env
	execCmd.Stdin = os.Stdin

	// Redirect container stdout/stderr to a log file so `thrive logs` can stream it.
	containerDir := filepath.Join("/run/thrive/containers", id)
	logPath := filepath.Join(containerDir, "logs")
	logFile, logErr := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if logErr != nil {
		log.Warn("runtime.Start: cannot open log file, falling back to stdout", telemetry.FieldError(logErr))
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
	} else {
		execCmd.Stdout = logFile
		execCmd.Stderr = logFile
	}

	// Full namespace isolation: PID, mount, UTS, IPC, network.
	log.Info("runtime.Start: setting up namespace flags")

	cloneFlags := uintptr(
		syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWNET,
	)
	sysProcAttr := &syscall.SysProcAttr{
		Cloneflags: cloneFlags,
	}
	// Chroot into the merged rootfs so the container sees its own filesystem.
	if rootfsPath != "" {
		sysProcAttr.Chroot = rootfsPath
		execCmd.Dir = "/"

		if dnsErr := network.WriteResolvConf(rootfsPath); dnsErr != nil {
			log.Warn("runtime.Start: WriteResolvConf failed (non-fatal)", telemetry.FieldError(dnsErr))
		}
	}

	// Pre-create volume mount destinations inside the container rootfs.
	// The actual bind-mounting happens in a ProcAttr.Sys callback after clone.
	for _, mnt := range cfg.Mounts {
		if rootfsPath != "" {
			dest := rootfsPath + mnt.Destination
			if mkErr := os.MkdirAll(dest, 0755); mkErr != nil {
				log.Warn("runtime.Start: mkdir volume dest", telemetry.FieldString("dest", dest), telemetry.FieldError(mkErr))
			}
		}
	}

	execCmd.SysProcAttr = sysProcAttr
	telemetry.Debug("runtime.Start: SysProcAttr configured", telemetry.FieldInt("cloneflags", int(cloneFlags)))

	if len(cfg.Secrets) > 0 {
		log.Info("runtime.Start: injecting secrets", telemetry.FieldString("containerID", id), telemetry.FieldInt("count", len(cfg.Secrets)))
		if err := secrets.Inject(id, cfg.Secrets); err != nil {
			log.Error("runtime.Start: secrets.Inject failed", telemetry.FieldString("containerID", id), telemetry.FieldError(err))
		}
	}

	log.Info("runtime.Start: starting process")
	err = execCmd.Start()
	if logFile != nil && logErr == nil {
		// Log file is now owned by the child; close our copy in the parent.
		logFile.Close()
	}
	if err != nil {
		log.Error("runtime.Start: exec.Start failed", telemetry.FieldError(err))
		return fmt.Errorf("runtime.Start: exec: %w", err)
	}

	pid := execCmd.Process.Pid
	log.Info("runtime.Start: process started", telemetry.FieldInt("pid", pid))

	// Set up container network isolation: bridge + veth pair + port forwarding.
	// Best-effort: network failures are logged but don't abort the container.
	var containerIP string
	if cfg.NetworkMode != "host" && cfg.NetworkMode != "none" {
		if bridgeErr := network.EnsureBridge(); bridgeErr != nil {
			log.Warn("runtime.Start: EnsureBridge failed", telemetry.FieldError(bridgeErr))
		} else {
			veth, vethErr := network.SetupVeth(id, pid)
			if vethErr != nil {
				log.Warn("runtime.Start: SetupVeth failed", telemetry.FieldError(vethErr))
			} else {
				containerIP = veth.ContainerIP
				log.Info("runtime.Start: network configured", telemetry.FieldString("ip", containerIP))
				// Apply port forwarding rules
				for _, pm := range cfg.Ports {
					if pfErr := network.AddPortForward(containerIP, pm.HostPort, pm.ContainerPort, pm.Protocol); pfErr != nil {
						log.Warn("runtime.Start: AddPortForward failed", telemetry.FieldError(pfErr))
					}
				}
			}
		}
	}

	// Wire cgroup v2 resource limits.
	if cgMgr, cgErr := cgroup.New(id); cgErr != nil {
		log.Warn("runtime.Start: cgroup.New failed", telemetry.FieldString("containerID", id), telemetry.FieldError(cgErr))
	} else {
		if applyErr := cgMgr.Apply(pid); applyErr != nil {
			log.Warn("runtime.Start: cgroup.Apply failed", telemetry.FieldInt("pid", pid), telemetry.FieldError(applyErr))
		}
		if cfg.Resources.MemoryLimit > 0 {
			if err := cgMgr.SetMemoryLimit(cfg.Resources.MemoryLimit); err != nil {
				log.Warn("runtime.Start: SetMemoryLimit failed", telemetry.FieldError(err))
			}
		}
		if cfg.Resources.CPUQuota > 0 {
			if err := cgMgr.SetCPUQuota(cfg.Resources.CPUQuota); err != nil {
				log.Warn("runtime.Start: SetCPUQuota failed", telemetry.FieldError(err))
			}
		}
	}

	// Save running state immediately so ps/kill work without waiting for exit.
	state.PID = pid
	state.Status = "running"
	if err := saveState(containerDir, state); err != nil {
		log.Error("runtime.Start: saveState (running) failed", telemetry.FieldString("containerID", id), telemetry.FieldError(err))
	}

	// Capture containerIP in closure for teardown
	capturedContainerIP := containerIP
	capturedCfg := cfg

	// Wait for process exit in the background; update state and clean up resources when done.
	go func() {
		if err := execCmd.Wait(); err != nil {
			log.Warn("runtime.Start: process wait error", telemetry.FieldString("containerID", id), telemetry.FieldError(err))
		}
		exitCode := 0
		if execCmd.ProcessState != nil {
			exitCode = execCmd.ProcessState.ExitCode()
		}
		telemetry.Debug("runtime.Start: process exited", telemetry.FieldInt("pid", pid), telemetry.FieldInt("exitCode", exitCode))

		state.Status = "stopped"
		state.ExitCode = exitCode
		if saveErr := saveState(containerDir, state); saveErr != nil {
			log.Error("runtime.Start: saveState (stopped) failed", telemetry.FieldString("containerID", id), telemetry.FieldError(saveErr))
		}

		// Tear down network (veth + port forward rules)
		if capturedContainerIP != "" {
			for _, pm := range capturedCfg.Ports {
				network.RemovePortForward(capturedContainerIP, pm.HostPort, pm.ContainerPort, pm.Protocol)
			}
			network.TeardownVeth(id)
		}

		if len(capturedCfg.Secrets) > 0 {
			if cleanErr := secrets.Cleanup(id); cleanErr != nil {
				log.Warn("runtime.Start: secrets.Cleanup failed", telemetry.FieldString("containerID", id), telemetry.FieldError(cleanErr))
			}
		}
		log.Info("runtime.Start: container exited", telemetry.FieldString("containerID", id), telemetry.FieldInt("exitCode", exitCode))
	}()

	log.Info("runtime.Start: container running", telemetry.FieldString("containerID", id), telemetry.FieldInt("pid", pid))
	return nil
}

// Kill sends a signal to the container's main process.
func Kill(ctx context.Context, id string, signal syscall.Signal) error {
	log := telemetry.Logger()
	log.Info("runtime.Kill: starting", telemetry.FieldString("containerID", id), telemetry.FieldInt("signal", int(signal)))

	state, err := loadState(id)
	if err != nil {
		log.Error("runtime.Kill: loadState failed", telemetry.FieldString("containerID", id), telemetry.FieldError(err))
		return fmt.Errorf("runtime.Kill: %w", err)
	}

	if state.PID <= 0 {
		log.Warn("runtime.Kill: container not running", telemetry.FieldString("containerID", id))
		return fmt.Errorf("runtime.Kill: container not running")
	}
	telemetry.Debug("runtime.Kill: target PID", telemetry.FieldInt("pid", state.PID))

	log.Info("runtime.Kill: sending signal", telemetry.FieldInt("pid", state.PID), telemetry.FieldInt("signal", int(signal)))
	if err := syscall.Kill(state.PID, signal); err != nil {
		log.Error("runtime.Kill: kill syscall failed", telemetry.FieldInt("pid", state.PID), telemetry.FieldError(err))
		return fmt.Errorf("runtime.Kill: kill: %w", err)
	}

	log.Info("runtime.Kill: completed successfully", telemetry.FieldString("containerID", id))
	return nil
}

// Delete removes the container's state and resources.
func Delete(ctx context.Context, id string) error {
	log := telemetry.Logger()
	log.Info("runtime.Delete: starting", telemetry.FieldString("containerID", id))

	containerDir := filepath.Join("/run/thrive/containers", id)
	log.Info("runtime.Delete: removing container directory", telemetry.FieldString("path", containerDir))

	if err := os.RemoveAll(containerDir); err != nil {
		log.Error("runtime.Delete: RemoveAll failed", telemetry.FieldString("path", containerDir), telemetry.FieldError(err))
		return fmt.Errorf("runtime.Delete: remove %s: %w", containerDir, err)
	}

	log.Info("runtime.Delete: completed successfully", telemetry.FieldString("containerID", id))
	return nil
}

// State returns the current state of a container.
func State(ctx context.Context, id string) (*ContainerState, error) {
	log := telemetry.Logger()
	log.Debug("runtime.State: loading state", telemetry.FieldString("containerID", id))

	state, err := loadState(id)
	if err != nil {
		log.Error("runtime.State: loadState failed", telemetry.FieldString("containerID", id), telemetry.FieldError(err))
		return nil, err
	}

	log.Debug("runtime.State: state loaded", telemetry.FieldString("containerID", id), telemetry.FieldString("status", state.Status))
	return state, nil
}

func loadState(id string) (*ContainerState, error) {
	statePath := filepath.Join("/run/thrive/containers", id, "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, fmt.Errorf("loadState: read %s: %w", statePath, err)
	}

	state := &ContainerState{}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("loadState: unmarshal: %w", err)
	}

	return state, nil
}

func saveState(dir string, state *ContainerState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "state.json"), data, 0644)
}
