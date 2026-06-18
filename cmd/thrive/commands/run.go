//go:build linux

package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

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
			}

			container, err := runtime.Create(ctx, cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating container: %v\n", err)
				os.Exit(1)
			}

			if err := runtime.Start(ctx, container.ID); err != nil {
				fmt.Fprintf(os.Stderr, "Error starting container: %v\n", err)
				os.Exit(1)
			}

			fmt.Println(container.ID)

			if detach {
				return
			}

			for {
				state, err := runtime.State(ctx, container.ID)
				if err != nil {
					break
				}
				if state.Status == "stopped" {
					if rm {
						runtime.Delete(ctx, container.ID)
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
	return cmd
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
