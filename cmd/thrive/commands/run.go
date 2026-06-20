//go:build linux

package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/runtime"
)

func RunCmd() *cobra.Command {
	var detach bool
	var rm bool
	var name string
	var envVars []string
	var secretNames []string
	var portSpecs []string
	var volumeSpecs []string
	var netMode string
	var tty bool
	var interactive bool

	cmd := &cobra.Command{
		Use:   "run [image] [command...]",
		Short: "Run a container",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			imageRef := args[0]

			fmt.Printf("Pulling image: %s\n", imageRef)
			img, err := image.Pull(ctx, imageRef, image.PullOptions{})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error pulling image: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Pulled: %s\n", img.Ref)

			containerID := name
			if containerID == "" {
				containerID = fmt.Sprintf("thrive-%d", os.Getpid())
			}

			ports, err := parsePortSpecs(portSpecs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing ports: %v\n", err)
				os.Exit(1)
			}

			cfg := runtime.ContainerConfig{
				ID:          containerID,
				Image:       img.Ref,
				Command:     args[1:],
				Env:         envVars,
				Secrets:     secretNames,
				Ports:       ports,
				Mounts:      parseVolumeSpecs(volumeSpecs),
				NetworkMode: netMode,
				TTY:         tty,
				Interactive: interactive || tty, // -t implies -i
			}

			container, err := runtime.Create(ctx, cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating container: %v\n", err)
				os.Exit(1)
			}

			ptm, err := runtime.Start(ctx, container.ID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error starting container: %v\n", err)
				os.Exit(1)
			}

			fmt.Println(container.ID)

			if detach {
				if ptm != nil {
					ptm.Close()
				}
				return
			}

			// Interactive TTY: relay I/O between the caller's terminal and the PTY master.
			if tty && ptm != nil {
				defer ptm.Close()

				// Raw mode so every keystroke goes straight to the container.
				oldTermios, rawErr := makeTermRaw(int(os.Stdin.Fd()))
				if rawErr == nil {
					defer restoreTermios(int(os.Stdin.Fd()), oldTermios)
				}

				// Forward window-resize signals into the PTY.
				sigwinch := make(chan os.Signal, 1)
				signal.Notify(sigwinch, syscall.SIGWINCH)
				go func() {
					for range sigwinch {
						if ws, wsErr := unix.IoctlGetWinsize(int(os.Stdin.Fd()), unix.TIOCGWINSZ); wsErr == nil {
							unix.IoctlSetWinsize(int(ptm.Fd()), unix.TIOCSWINSZ, ws) //nolint:errcheck
						}
					}
				}()

				go io.Copy(ptm, os.Stdin) //nolint:errcheck
				io.Copy(os.Stdout, ptm)   //nolint:errcheck

				signal.Stop(sigwinch)
				close(sigwinch)

				state, _ := runtime.State(ctx, container.ID)
				if rm {
					runtime.Delete(ctx, container.ID) //nolint:errcheck
				}
				if state != nil {
					os.Exit(state.ExitCode)
				}
				return
			}

			// Non-TTY foreground: poll until stopped, then stream logs.
			for {
				state, err := runtime.State(ctx, container.ID)
				if err != nil {
					break
				}
				if state.Status == "stopped" {
					logPath := "/run/thrive/containers/" + container.ID + "/logs"
					if logData, readErr := os.ReadFile(logPath); readErr == nil {
						os.Stdout.Write(logData)
					}
					if rm {
						runtime.Delete(ctx, container.ID) //nolint:errcheck
					}
					os.Exit(state.ExitCode)
				}
				time.Sleep(100 * time.Millisecond)
			}
		},
	}

	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "Run container in background")
	cmd.Flags().BoolVar(&rm, "rm", false, "Remove container on exit")
	cmd.Flags().StringVar(&name, "name", "", "Container name")
	cmd.Flags().StringArrayVarP(&envVars, "env", "e", nil, "Set environment variables")
	cmd.Flags().StringArrayVar(&secretNames, "secret", nil, "Secrets to inject")
	cmd.Flags().StringArrayVarP(&portSpecs, "publish", "p", nil, "Publish port(s): host:container[/proto]")
	cmd.Flags().StringArrayVarP(&volumeSpecs, "volume", "v", nil, "Bind mount: /host:/container")
	cmd.Flags().StringVar(&netMode, "network", "", "Network mode (host, none, or default bridge)")
	cmd.Flags().BoolVarP(&tty, "tty", "t", false, "Allocate a pseudo-TTY")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Keep stdin open")
	// Stop flag parsing after the image name so container commands can include flags.
	cmd.Flags().SetInterspersed(false)
	return cmd
}

// makeTermRaw puts fd into raw terminal mode and returns the previous state.
func makeTermRaw(fd int) (*unix.Termios, error) {
	t, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, err
	}
	raw := *t
	raw.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP |
		unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	raw.Oflag &^= unix.OPOST
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Cflag &^= unix.CSIZE | unix.PARENB
	raw.Cflag |= unix.CS8
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(fd, unix.TCSETSF, &raw); err != nil {
		return nil, err
	}
	return t, nil
}

func restoreTermios(fd int, state *unix.Termios) {
	unix.IoctlSetTermios(fd, unix.TCSETSF, state) //nolint:errcheck
}

func parsePortSpecs(specs []string) ([]runtime.PortMapping, error) {
	var ports []runtime.PortMapping
	for _, spec := range specs {
		proto := "tcp"
		if idx := indexByte(spec, '/'); idx >= 0 {
			proto = spec[idx+1:]
			spec = spec[:idx]
		}
		idx := indexByte(spec, ':')
		if idx < 0 {
			return nil, fmt.Errorf("invalid port spec %q: expected host:container", spec)
		}
		hostPort, err := parsePort(spec[:idx])
		if err != nil {
			return nil, fmt.Errorf("invalid host port in %q: %w", spec, err)
		}
		ctrPort, err := parsePort(spec[idx+1:])
		if err != nil {
			return nil, fmt.Errorf("invalid container port in %q: %w", spec, err)
		}
		ports = append(ports, runtime.PortMapping{
			HostPort:      hostPort,
			ContainerPort: ctrPort,
			Protocol:      proto,
		})
	}
	return ports, nil
}

func parseVolumeSpecs(specs []string) []runtime.Mount {
	var mounts []runtime.Mount
	for _, spec := range specs {
		idx := indexByte(spec, ':')
		if idx < 0 {
			mounts = append(mounts, runtime.Mount{Source: spec, Destination: spec, Type: "bind"})
			continue
		}
		mounts = append(mounts, runtime.Mount{
			Source:      spec[:idx],
			Destination: spec[idx+1:],
			Type:        "bind",
			Options:     []string{"rbind"},
		})
	}
	return mounts
}

func parsePort(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("non-numeric character in port")
		}
		n = n*10 + int(c-'0')
	}
	if n <= 0 || n > 65535 {
		return 0, fmt.Errorf("port out of range: %d", n)
	}
	return n, nil
}

func indexByte(s string, b byte) int {
	for i := range s {
		if s[i] == b {
			return i
		}
	}
	return -1
}
