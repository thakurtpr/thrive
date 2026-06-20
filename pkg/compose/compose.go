//go:build linux

// Package compose provides docker-compose.yml compatible service orchestration
// using Thrive's native container runtime.
package compose

import (
	"context"
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/runtime"
)

// ComposeFile represents a docker-compose.yml or thrive-compose.yml manifest.
type ComposeFile struct {
	Version  string                 `yaml:"version"`
	Services map[string]*ServiceDef `yaml:"services"`
	Networks map[string]*NetworkDef `yaml:"networks,omitempty"`
	Volumes  map[string]*VolumeDef  `yaml:"volumes,omitempty"`
}

// ServiceDef mirrors the docker-compose service spec.
type ServiceDef struct {
	Image       string            `yaml:"image"`
	Command     []string          `yaml:"command,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty"`
	Ports       []string          `yaml:"ports,omitempty"`
	Volumes     []string          `yaml:"volumes,omitempty"`
	DependsOn   []string          `yaml:"depends_on,omitempty"`
	Restart     string            `yaml:"restart,omitempty"`
}

// NetworkDef and VolumeDef satisfy YAML parsing for those top-level keys.
type NetworkDef struct{ Driver string `yaml:"driver,omitempty"` }
type VolumeDef struct{ Driver string `yaml:"driver,omitempty"` }

// ServiceStatus holds runtime status of a compose service container.
type ServiceStatus struct {
	Name   string
	ID     string
	Status string
}

// Load reads and parses a compose file.
func Load(path string) (*ComposeFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("compose.Load: read %s: %w", path, err)
	}

	var cf ComposeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("compose.Load: parse %s: %w", path, err)
	}

	if len(cf.Services) == 0 {
		return nil, fmt.Errorf("compose.Load: no services defined in %s", path)
	}

	return &cf, nil
}

// Up starts all services in dependency order.
func Up(ctx context.Context, cf *ComposeFile, projectName string) error {
	order, err := startOrder(cf)
	if err != nil {
		return fmt.Errorf("compose.Up: %w", err)
	}

	for _, name := range order {
		svc := cf.Services[name]
		if err := startService(ctx, projectName, name, svc); err != nil {
			return fmt.Errorf("compose.Up: service %s: %w", name, err)
		}
		fmt.Printf("  started %s\n", name)
	}
	return nil
}

// Down stops and removes all service containers in parallel.
func Down(ctx context.Context, cf *ComposeFile, projectName string) error {
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup

	for name := range cf.Services {
		wg.Add(1)
		go func(svcName string) {
			defer wg.Done()
			id := containerName(projectName, svcName)
			state, err := runtime.State(ctx, id)
			if err != nil {
				return
			}
			if state.Status == "running" {
				runtime.Kill(ctx, id, 15) // SIGTERM
			}
			if err := runtime.Delete(ctx, id); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("service %s: %w", svcName, err)
				}
				mu.Unlock()
			}
			fmt.Printf("  stopped %s\n", svcName)
		}(name)
	}
	wg.Wait()
	return firstErr
}

// Ps lists status of all service containers.
func Ps(ctx context.Context, cf *ComposeFile, projectName string) ([]ServiceStatus, error) {
	var statuses []ServiceStatus
	for name := range cf.Services {
		id := containerName(projectName, name)
		state, err := runtime.State(ctx, id)
		status := "not started"
		if err == nil {
			status = state.Status
		}
		statuses = append(statuses, ServiceStatus{
			Name:   name,
			ID:     id,
			Status: status,
		})
	}
	return statuses, nil
}

func startService(ctx context.Context, projectName, name string, svc *ServiceDef) error {
	if svc.Image == "" {
		return fmt.Errorf("service %s has no image", name)
	}

	fmt.Printf("  pulling %s\n", svc.Image)
	if _, err := image.Pull(ctx, svc.Image, image.PullOptions{}); err != nil {
		return fmt.Errorf("pull %s: %w", svc.Image, err)
	}

	id := containerName(projectName, name)

	var envVars []string
	for k, v := range svc.Environment {
		envVars = append(envVars, k+"="+v)
	}

	ports, _ := parseComposePorts(svc.Ports)
	mounts := parseComposeVolumes(svc.Volumes)

	cfg := runtime.ContainerConfig{
		ID:      id,
		Image:   svc.Image,
		Command: svc.Command,
		Env:     envVars,
		Ports:   ports,
		Mounts:  mounts,
	}

	if _, err := runtime.Create(ctx, cfg); err != nil {
		return fmt.Errorf("create: %w", err)
	}
	if _, err := runtime.Start(ctx, id); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	return nil
}

func containerName(project, service string) string {
	return project + "-" + service + "-1"
}

// startOrder returns service names in dependency-first topological order.
func startOrder(cf *ComposeFile) ([]string, error) {
	visited := make(map[string]bool)
	inStack := make(map[string]bool)
	var result []string

	var visit func(name string) error
	visit = func(name string) error {
		if inStack[name] {
			return fmt.Errorf("circular dependency at service %s", name)
		}
		if visited[name] {
			return nil
		}
		inStack[name] = true
		svc, ok := cf.Services[name]
		if !ok {
			return fmt.Errorf("service %s not found", name)
		}
		for _, dep := range svc.DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}
		inStack[name] = false
		visited[name] = true
		result = append(result, name)
		return nil
	}

	for name := range cf.Services {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func parseComposePorts(specs []string) ([]runtime.PortMapping, error) {
	var ports []runtime.PortMapping
	for _, spec := range specs {
		proto := "tcp"
		for i := len(spec) - 1; i >= 0; i-- {
			if spec[i] == '/' {
				proto = spec[i+1:]
				spec = spec[:i]
				break
			}
		}
		idx := -1
		for i, c := range spec {
			if c == ':' {
				idx = i
				break
			}
		}
		if idx < 0 {
			continue
		}
		hp := parsePortNum(spec[:idx])
		cp := parsePortNum(spec[idx+1:])
		if hp > 0 && cp > 0 {
			ports = append(ports, runtime.PortMapping{
				HostPort:      hp,
				ContainerPort: cp,
				Protocol:      proto,
			})
		}
	}
	return ports, nil
}

func parseComposeVolumes(specs []string) []runtime.Mount {
	var mounts []runtime.Mount
	for _, spec := range specs {
		idx := -1
		for i, c := range spec {
			if c == ':' {
				idx = i
				break
			}
		}
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

func parsePortNum(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
