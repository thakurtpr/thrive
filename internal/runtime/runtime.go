//go:build linux
// +build linux

package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"

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
// When cfg.TTY is set, Start allocates a pseudo-terminal and returns the master
// side so the caller can relay I/O. Otherwise it returns nil, nil on success.
func Start(ctx context.Context, id string) (*os.File, error) {
	log := telemetry.Logger()
	log.Info("runtime.Start: starting", telemetry.FieldString("containerID", id))

	state, err := loadState(id)
	if err != nil {
		log.Error("runtime.Start: loadState failed", telemetry.FieldString("containerID", id), telemetry.FieldError(err))
		return nil, fmt.Errorf("runtime.Start: %w", err)
	}
	telemetry.Debug("runtime.Start: state loaded", telemetry.FieldString("containerID", id), telemetry.FieldString("status", state.Status))

	if state.Status != "created" {
		log.Warn("runtime.Start: container not in created state",
			telemetry.FieldString("containerID", id),
			telemetry.FieldString("status", state.Status))
		return nil, fmt.Errorf("runtime.Start: container already started or deleted")
	}
	telemetry.Debug("runtime.Start: status check passed", telemetry.FieldString("containerID", id))

	configPath := filepath.Join("/run/thrive/containers", id, "config.json")
	log.Info("runtime.Start: reading config file", telemetry.FieldString("configPath", configPath))

	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Error("runtime.Start: read config failed", telemetry.FieldString("configPath", configPath), telemetry.FieldError(err))
		return nil, fmt.Errorf("runtime.Start: read config: %w", err)
	}

	cfg := &ContainerConfig{}
	if err := json.Unmarshal(data, cfg); err != nil {
		log.Error("runtime.Start: unmarshal config failed", telemetry.FieldError(err))
		return nil, fmt.Errorf("runtime.Start: unmarshal config: %w", err)
	}
	telemetry.Debug("runtime.Start: config unmarshaled", telemetry.FieldString("image", cfg.Image))

	log.Info("runtime.Start: mounting rootfs", telemetry.FieldString("image", cfg.Image))
	rootfsPath, mountErr := image.Mount(ctx, cfg.Image, id)
	if mountErr != nil {
		log.Warn("runtime.Start: rootfs mount failed, running without chroot",
			telemetry.FieldString("containerID", id), telemetry.FieldError(mountErr))
		rootfsPath = ""
	} else {
		log.Info("runtime.Start: rootfs mounted", telemetry.FieldString("rootfs", rootfsPath))
	}

	cmd := cfg.Command
	if len(cmd) == 0 {
		cmd = []string{"/bin/sh"}
	}

	// Resolve non-absolute binary names within the rootfs.
	binary := cmd[0]
	if rootfsPath != "" && len(binary) > 0 && binary[0] != '/' {
		found := false
		for _, dir := range []string{
			"/usr/local/go/bin",
			"/usr/local/sbin", "/usr/local/bin",
			"/usr/sbin", "/usr/bin",
			"/sbin", "/bin",
		} {
			if _, statErr := os.Stat(filepath.Join(rootfsPath, dir, binary)); statErr == nil {
				binary = filepath.Join(dir, binary)
				found = true
				break
			}
		}
		if !found {
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

	imageEnv, _, _ := image.ReadManifest(cfg.Image)
	execCmd.Env = append(imageEnv, cfg.Env...)

	// Derive GOROOT for Go toolchain binaries that use /proc/self/exe to locate it.
	if strings.HasSuffix(binary, "/bin/go") && !envContains(execCmd.Env, "GOROOT=") {
		execCmd.Env = append(execCmd.Env, "GOROOT="+strings.TrimSuffix(binary, "/bin/go"))
	}

	// When running inside the LinuxKit VM (THRIVE_VSOCK_PORT is set), the VM
	// itself provides process isolation and the containerd seccomp profile
	// blocks namespace clone flags. Use chroot-only isolation instead.
	insideVM := os.Getenv("THRIVE_VSOCK_PORT") != ""
	var cloneFlags uintptr
	if !insideVM {
		cloneFlags = uintptr(
			syscall.CLONE_NEWPID |
				syscall.CLONE_NEWNS |
				syscall.CLONE_NEWUTS |
				syscall.CLONE_NEWIPC |
				syscall.CLONE_NEWNET,
		)
	}
	sysProcAttr := &syscall.SysProcAttr{
		Cloneflags: cloneFlags,
	}

	if rootfsPath != "" {
		sysProcAttr.Chroot = rootfsPath
		execCmd.Dir = "/"
		if dnsErr := network.WriteResolvConf(rootfsPath); dnsErr != nil {
			log.Warn("runtime.Start: WriteResolvConf failed (non-fatal)", telemetry.FieldError(dnsErr))
		}
	}

	// Bind-mount volumes in the parent namespace before the fork.
	// CLONE_NEWNS copies the parent's mount table into the child; any bind mounts
	// done here are therefore visible inside the container after the chroot.
	// We lazy-unmount them from the parent namespace right after Start so the host
	// mount table stays clean.
	var parentBinds []string
	if rootfsPath != "" {
		for _, mnt := range cfg.Mounts {
			dest := filepath.Join(rootfsPath, mnt.Destination)
			// Match source type: directory → mkdir, file → mkfile.
			if srcInfo, statErr := os.Stat(mnt.Source); statErr == nil && !srcInfo.IsDir() {
				os.MkdirAll(filepath.Dir(dest), 0755)
				if f, createErr := os.OpenFile(dest, os.O_CREATE|os.O_RDONLY, 0644); createErr == nil {
					f.Close()
				}
			} else {
				os.MkdirAll(dest, 0755)
			}
			if bindErr := syscall.Mount(mnt.Source, dest, "", syscall.MS_BIND|syscall.MS_REC, ""); bindErr != nil {
				log.Warn("runtime.Start: bind mount failed",
					telemetry.FieldString("src", mnt.Source),
					telemetry.FieldString("dest", dest),
					telemetry.FieldError(bindErr))
				continue
			}
			parentBinds = append(parentBinds, dest)
			log.Info("runtime.Start: bound volume", telemetry.FieldString("src", mnt.Source), telemetry.FieldString("dest", mnt.Destination))
		}
	}

	containerDir := filepath.Join("/run/thrive/containers", id)

	// Wire stdio: PTY for -t, direct for -i, log file for background.
	var ptmx *os.File
	var ptsSlave *os.File // slave side; closed in parent after Start
	if cfg.TTY {
		var ptyErr error
		ptmx, ptsSlave, ptyErr = setupPTY(execCmd, sysProcAttr)
		if ptyErr != nil {
			return nil, fmt.Errorf("runtime.Start: PTY setup: %w", ptyErr)
		}
	} else if cfg.Interactive {
		execCmd.Stdin = os.Stdin
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		execCmd.SysProcAttr = sysProcAttr
	} else {
		execCmd.Stdin = os.Stdin
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
		execCmd.SysProcAttr = sysProcAttr
		if logFile != nil {
			defer logFile.Close()
		}
	}

	if len(cfg.Secrets) > 0 {
		log.Info("runtime.Start: injecting secrets", telemetry.FieldString("containerID", id), telemetry.FieldInt("count", len(cfg.Secrets)))
		if sErr := secrets.Inject(id, cfg.Secrets); sErr != nil {
			log.Error("runtime.Start: secrets.Inject failed", telemetry.FieldString("containerID", id), telemetry.FieldError(sErr))
		}
	}

	log.Info("runtime.Start: starting process")
	hasNamespaces := cloneFlags != 0
	if startErr := execCmd.Start(); startErr != nil {
		// If namespace creation was blocked (e.g. thrived running inside a
		// container runtime with seccomp restrictions), retry with chroot-only
		// isolation. The VM itself provides process isolation.
		if cloneFlags != 0 && isPermissionError(startErr) {
			log.Warn("runtime.Start: namespace creation blocked, retrying chroot-only", telemetry.FieldError(startErr))
			hasNamespaces = false
			fallbackAttr := &syscall.SysProcAttr{}
			if rootfsPath != "" {
				fallbackAttr.Chroot = rootfsPath
			}
			if ptmx != nil {
				ptmx.Close()
				ptmx = nil
			}
			if ptsSlave != nil {
				ptsSlave.Close()
				ptsSlave = nil
			}
			execCmd2 := exec.Command(execCmd.Path, execCmd.Args[1:]...)
			execCmd2.Args = execCmd.Args
			execCmd2.Env = execCmd.Env
			execCmd2.Stdin = execCmd.Stdin
			execCmd2.Stdout = execCmd.Stdout
			execCmd2.Stderr = execCmd.Stderr
			execCmd2.Dir = execCmd.Dir
			if cfg.TTY {
				var ptyErr error
				ptmx, ptsSlave, ptyErr = setupPTY(execCmd2, fallbackAttr)
				if ptyErr != nil {
					return nil, fmt.Errorf("runtime.Start: PTY setup (fallback): %w", ptyErr)
				}
			} else {
				execCmd2.SysProcAttr = fallbackAttr
			}
			execCmd = execCmd2
			if startErr2 := execCmd.Start(); startErr2 != nil {
				if ptmx != nil {
					ptmx.Close()
				}
				if ptsSlave != nil {
					ptsSlave.Close()
				}
				log.Error("runtime.Start: exec.Start (fallback) failed", telemetry.FieldError(startErr2))
				return nil, fmt.Errorf("runtime.Start: exec: %w", startErr2)
			}
		} else {
			if ptmx != nil {
				ptmx.Close()
			}
			if ptsSlave != nil {
				ptsSlave.Close()
			}
			log.Error("runtime.Start: exec.Start failed", telemetry.FieldError(startErr))
			return nil, fmt.Errorf("runtime.Start: exec: %w", startErr)
		}
	}
	// Parent no longer needs the slave side; child inherited it via FD 0/1/2.
	if ptsSlave != nil {
		ptsSlave.Close()
	}

	// Detach bind mounts from the parent namespace only when CLONE_NEWNS succeeded.
	// In chroot-only mode the mounts stay in the parent namespace (cleaned up on delete).
	if hasNamespaces {
		for _, dest := range parentBinds {
			if umErr := syscall.Unmount(dest, syscall.MNT_DETACH); umErr != nil {
				log.Warn("runtime.Start: parent-side unmount failed", telemetry.FieldString("dest", dest), telemetry.FieldError(umErr))
			}
		}
	}

	pid := execCmd.Process.Pid
	log.Info("runtime.Start: process started", telemetry.FieldInt("pid", pid))

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
				for _, pm := range cfg.Ports {
					if pfErr := network.AddPortForward(containerIP, pm.HostPort, pm.ContainerPort, pm.Protocol); pfErr != nil {
						log.Warn("runtime.Start: AddPortForward failed", telemetry.FieldError(pfErr))
					}
				}
			}
		}
	}

	if cgMgr, cgErr := cgroup.New(id); cgErr != nil {
		log.Warn("runtime.Start: cgroup.New failed", telemetry.FieldString("containerID", id), telemetry.FieldError(cgErr))
	} else {
		if applyErr := cgMgr.Apply(pid); applyErr != nil {
			log.Warn("runtime.Start: cgroup.Apply failed", telemetry.FieldInt("pid", pid), telemetry.FieldError(applyErr))
		}
		if cfg.Resources.MemoryLimit > 0 {
			if limErr := cgMgr.SetMemoryLimit(cfg.Resources.MemoryLimit); limErr != nil {
				log.Warn("runtime.Start: SetMemoryLimit failed", telemetry.FieldError(limErr))
			}
		}
		if cfg.Resources.CPUQuota > 0 {
			if quotaErr := cgMgr.SetCPUQuota(cfg.Resources.CPUQuota); quotaErr != nil {
				log.Warn("runtime.Start: SetCPUQuota failed", telemetry.FieldError(quotaErr))
			}
		}
	}

	state.PID = pid
	state.Status = "running"
	state.HasNamespaces = hasNamespaces
	state.RootfsPath = rootfsPath
	if err := saveState(containerDir, state); err != nil {
		log.Error("runtime.Start: saveState (running) failed", telemetry.FieldString("containerID", id), telemetry.FieldError(err))
	}

	capturedContainerIP := containerIP
	capturedCfg := cfg

	go func() {
		if waitErr := execCmd.Wait(); waitErr != nil {
			log.Warn("runtime.Start: process wait error", telemetry.FieldString("containerID", id), telemetry.FieldError(waitErr))
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
	return ptmx, nil
}

// setupPTY allocates a pseudo-terminal pair, wires the slave to the command's
// stdio, and configures SysProcAttr for session leadership.
// Returns (master, slave, err). The caller must close slave after execCmd.Start()
// and close master when the interactive session ends.
func setupPTY(execCmd *exec.Cmd, attr *syscall.SysProcAttr) (ptmx *os.File, pts *os.File, err error) {
	// Open PTY master; O_CLOEXEC keeps it out of the child process.
	ptmxFd, openErr := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if openErr != nil {
		return nil, nil, fmt.Errorf("open /dev/ptmx: %w", openErr)
	}
	ptmx = os.NewFile(uintptr(ptmxFd), "/dev/ptmx")

	// Unlock the slave PTY (no-op on Linux ≥ 4.1 but required for POSIX).
	unix.IoctlSetInt(ptmxFd, unix.TIOCSPTLCK, 0) //nolint:errcheck

	// TIOCGPTN returns the slave PTY number; construct /dev/pts/<n> from it.
	var ptyno uint32
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(ptmxFd), unix.TIOCGPTN, uintptr(unsafe.Pointer(&ptyno))); errno != 0 {
		ptmx.Close()
		return nil, nil, fmt.Errorf("TIOCGPTN: %w", errno)
	}
	slaveName := fmt.Sprintf("/dev/pts/%d", ptyno)

	// O_NOCTTY so the parent opening the slave doesn't steal the controlling terminal.
	pts, ptsErr := os.OpenFile(slaveName, os.O_RDWR|syscall.O_NOCTTY, 0)
	if ptsErr != nil {
		ptmx.Close()
		return nil, nil, fmt.Errorf("open pts %s: %w", slaveName, ptsErr)
	}

	// Mirror parent terminal size into the PTY so the shell renders correctly.
	if ws, wsErr := unix.IoctlGetWinsize(int(os.Stdin.Fd()), unix.TIOCGWINSZ); wsErr == nil {
		unix.IoctlSetWinsize(ptmxFd, unix.TIOCSWINSZ, ws) //nolint:errcheck
	}

	execCmd.Stdin = pts
	execCmd.Stdout = pts
	execCmd.Stderr = pts

	attr.Setsid = true
	attr.Setctty = true
	attr.Ctty = 0 // FD 0 (stdin = pts) becomes the controlling terminal in the child
	execCmd.SysProcAttr = attr

	return ptmx, pts, nil
}

// isPermissionError returns true when err is an EPERM or EACCES syscall error.
// Used to detect namespace creation failures inside constrained environments.
func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	var errno syscall.Errno
	if strings.Contains(err.Error(), "operation not permitted") ||
		strings.Contains(err.Error(), "permission denied") {
		return true
	}
	return errors.As(err, &errno) && (errno == syscall.EPERM || errno == syscall.EACCES)
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

// envContains reports whether any entry in env has the given prefix.
func envContains(env []string, prefix string) bool {
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return true
		}
	}
	return false
}
