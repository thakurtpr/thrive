//go:build linux

// Package compose provides docker-compose.yml compatible service orchestration
// using Thrive's native container runtime.
package compose

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/network"
	"github.com/thakurprasadrout/thrive/internal/runtime"
	"github.com/thakurprasadrout/thrive/internal/volume"
	"github.com/thakurprasadrout/thrive/pkg/build"
	"github.com/thakurprasadrout/thrive/pkg/dockerfile"
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
	Build       *BuildDef         `yaml:"build,omitempty"`
	Command     []string          `yaml:"command,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty"`
	Ports       []string          `yaml:"ports,omitempty"`
	Volumes     []string          `yaml:"volumes,omitempty"`
	Networks    []string          `yaml:"networks,omitempty"`
	DependsOn   []string          `yaml:"depends_on,omitempty"`
	Restart     string            `yaml:"restart,omitempty"`
}

// BuildDef mirrors the compose build section (string or map form).
type BuildDef struct {
	Context    string            `yaml:"-"`
	Dockerfile string            `yaml:"dockerfile,omitempty"`
	Args       map[string]string `yaml:"args,omitempty"`
}

// UnmarshalYAML accepts `build: ./dir` and the mapping form.
func (b *BuildDef) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err == nil {
		b.Context = s
		return nil
	}
	type buildMap struct {
		Context    string            `yaml:"context"`
		Dockerfile string            `yaml:"dockerfile"`
		Args       map[string]string `yaml:"args"`
	}
	var m buildMap
	if err := value.Decode(&m); err != nil {
		return err
	}
	b.Context = m.Context
	b.Dockerfile = m.Dockerfile
	b.Args = m.Args
	return nil
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
	return UpScaled(ctx, cf, projectName, nil)
}

// UpScaled starts services with per-service replica counts.
func UpScaled(ctx context.Context, cf *ComposeFile, projectName string, scales map[string]int) error {
	if err := ensureResources(cf); err != nil {
		return fmt.Errorf("compose.Up: %w", err)
	}
	order, err := startOrder(cf)
	if err != nil {
		return fmt.Errorf("compose.Up: %w", err)
	}

	for _, name := range order {
		svc := cf.Services[name]
		replicas := scales[name]
		if replicas < 1 {
			replicas = 1
		}
		for i := 1; i <= replicas; i++ {
			if err := startServiceAt(ctx, projectName, name, svc, i); err != nil {
				return fmt.Errorf("compose.Up: service %s: %w", name, err)
			}
			fmt.Printf("  started %s (%d/%d)\n", name, i, replicas)
		}
	}
	return nil
}

// ensureResources creates declared networks and volumes.
func ensureResources(cf *ComposeFile) error {
	for netName := range cf.Networks {
		if _, err := network.InspectNetwork(netName); err != nil {
			if _, err := network.CreateNetwork(netName, "", ""); err != nil {
				return fmt.Errorf("network %s: %w", netName, err)
			}
		}
	}
	for volName := range cf.Volumes {
		if _, err := volume.Ensure(volName); err != nil {
			return fmt.Errorf("volume %s: %w", volName, err)
		}
	}
	return nil
}

// Build builds services with a build section from their contexts.
func Build(ctx context.Context, cf *ComposeFile, composeDir string, targets []string) error {
	for _, name := range filterServices(cf, targets) {
		svc := cf.Services[name]
		if svc.Build == nil || svc.Build.Context == "" {
			continue
		}
		contextDir := svc.Build.Context
		if !filepath.IsAbs(contextDir) {
			contextDir = filepath.Join(composeDir, contextDir)
		}
		dockerfilePath := svc.Build.Dockerfile
		if dockerfilePath == "" {
			dockerfilePath = "Dockerfile"
		}
		if !filepath.IsAbs(dockerfilePath) {
			dockerfilePath = filepath.Join(contextDir, dockerfilePath)
		}
		res, err := dockerfile.ParseFile(dockerfilePath, svc.Build.Args)
		if err != nil {
			return fmt.Errorf("compose.Build: service %s: %w", name, err)
		}
		for _, w := range res.Warnings {
			fmt.Printf("  build %s: warning: %s\n", name, w)
		}
		tag := svc.Image
		if tag == "" {
			tag = name + ":latest"
			svc.Image = tag
		}
		result, err := build.Execute(ctx, res.Graph, build.BuildOptions{
			Tag: tag, ContextDir: contextDir,
		})
		if err != nil {
			return fmt.Errorf("compose.Build: service %s: %w", name, err)
		}
		fmt.Printf("  built %s (%d steps)\n", result.ImageID, result.Steps)
	}
	return nil
}

// Pull pulls all service images.
func Pull(ctx context.Context, cf *ComposeFile, targets []string) error {
	for _, name := range filterServices(cf, targets) {
		svc := cf.Services[name]
		if svc.Image == "" {
			continue
		}
		fmt.Printf("  pulling %s\n", svc.Image)
		if _, err := image.Pull(ctx, svc.Image, image.PullOptions{}); err != nil {
			return fmt.Errorf("compose.Pull: service %s: %w", name, err)
		}
	}
	return nil
}

// Stop stops service containers (SIGTERM, escalates to SIGKILL).
func Stop(ctx context.Context, cf *ComposeFile, projectName string, targets []string, timeout time.Duration) error {
	for _, name := range filterServices(cf, targets) {
		for _, id := range serviceIDs(projectName, name) {
			state, err := runtime.State(ctx, id)
			if err != nil || state.Status != "running" {
				continue
			}
			_ = runtime.Kill(ctx, id, syscall.SIGTERM)
			deadline := time.Now().Add(timeout)
			for time.Now().Before(deadline) {
				time.Sleep(200 * time.Millisecond)
				s, err := runtime.State(ctx, id)
				if err != nil || s.Status == "stopped" {
					break
				}
			}
			_ = runtime.Kill(ctx, id, syscall.SIGKILL)
			// Wait (bounded) until the stopped state is observable: without
			// this, a racing `ps` still shows running after stop returns.
			waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, _ = runtime.Wait(waitCtx, id)
			cancel()
			fmt.Printf("  stopped %s\n", id)
		}
	}
	return nil
}

// Start starts existing (stopped) service containers.
func Start(ctx context.Context, cf *ComposeFile, projectName string, targets []string) error {
	for _, name := range filterServices(cf, targets) {
		for _, id := range serviceIDs(projectName, name) {
			state, err := runtime.State(ctx, id)
			if err != nil {
				return fmt.Errorf("compose.Start: service %s: %w", name, err)
			}
			if state.Status == "running" {
				continue
			}
			// Reset created state so runtime.Start accepts it.
			if state.Status != "created" {
				if err := resetToCreated(id); err != nil {
					return fmt.Errorf("compose.Start: service %s: %w", name, err)
				}
			}
			if _, err := runtime.Start(ctx, id); err != nil {
				return fmt.Errorf("compose.Start: service %s: %w", name, err)
			}
			fmt.Printf("  started %s\n", id)
		}
	}
	return nil
}

// Kill sends a signal to service containers.
func Kill(ctx context.Context, cf *ComposeFile, projectName string, targets []string, sig syscall.Signal) error {
	for _, name := range filterServices(cf, targets) {
		for _, id := range serviceIDs(projectName, name) {
			state, err := runtime.State(ctx, id)
			if err != nil || state.Status != "running" {
				continue
			}
			if err := runtime.Kill(ctx, id, sig); err != nil {
				return fmt.Errorf("compose.Kill: service %s: %w", name, err)
			}
			fmt.Printf("  killed %s\n", id)
		}
	}
	return nil
}

// Restart restarts service containers.
func Restart(ctx context.Context, cf *ComposeFile, projectName string, targets []string, timeout time.Duration) error {
	if err := Stop(ctx, cf, projectName, targets, timeout); err != nil {
		return err
	}
	return Start(ctx, cf, projectName, targets)
}

// Rm removes stopped service containers.
func Rm(ctx context.Context, cf *ComposeFile, projectName string, targets []string, force bool) error {
	for _, name := range filterServices(cf, targets) {
		for _, id := range serviceIDs(projectName, name) {
			state, err := runtime.State(ctx, id)
			if err != nil {
				continue
			}
			if state.Status == "running" {
				if !force {
					return fmt.Errorf("compose.Rm: service %s is running (use --force)", name)
				}
				_ = runtime.Kill(ctx, id, syscall.SIGKILL)
			}
			if err := runtime.Delete(ctx, id); err != nil {
				return fmt.Errorf("compose.Rm: service %s: %w", name, err)
			}
			fmt.Printf("  removed %s\n", id)
		}
	}
	return nil
}

// Config renders the normalised compose file (docker compose config parity).
func Config(cf *ComposeFile) (string, error) {
	data, err := yaml.Marshal(cf)
	if err != nil {
		return "", fmt.Errorf("compose.Config: %w", err)
	}
	return string(data), nil
}

// filterServices returns target names, or all services when empty.
func filterServices(cf *ComposeFile, targets []string) []string {
	if len(targets) == 0 {
		var all []string
		for name := range cf.Services {
			all = append(all, name)
		}
		return all
	}
	var out []string
	for _, t := range targets {
		if _, ok := cf.Services[t]; ok {
			out = append(out, t)
		}
	}
	return out
}

func resetToCreated(id string) error {
	statePath := filepath.Join("/run/thrive/containers", id, "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return err
	}
	var state map[string]any
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	state["status"] = "created"
	updated, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(statePath, updated, 0644)
}

// Down stops and removes all service containers in parallel.
func Down(ctx context.Context, cf *ComposeFile, projectName string) error {
	var firstErr error

	for name := range cf.Services {
		for _, id := range serviceIDs(projectName, name) {
			state, err := runtime.State(ctx, id)
			if err != nil {
				continue
			}
			if state.Status == "running" {
				runtime.Kill(ctx, id, 15) // SIGTERM
			}
			if err := runtime.Delete(ctx, id); err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("service %s: %w", name, err)
				}
			}
			fmt.Printf("  stopped %s\n", id)
		}
	}
	return firstErr
}

// Ps lists status of all service containers.
func Ps(ctx context.Context, cf *ComposeFile, projectName string) ([]ServiceStatus, error) {
	var statuses []ServiceStatus
	for name := range cf.Services {
		for _, id := range serviceIDs(projectName, name) {
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
	}
	return statuses, nil
}

func startService(ctx context.Context, projectName, name string, svc *ServiceDef) error {
	return startServiceAt(ctx, projectName, name, svc, 1)
}

func startServiceAt(ctx context.Context, projectName, name string, svc *ServiceDef, index int) error {
	if svc.Image == "" {
		return fmt.Errorf("service %s has no image", name)
	}

	fmt.Printf("  pulling %s\n", svc.Image)
	if _, err := image.Pull(ctx, svc.Image, image.PullOptions{}); err != nil {
		return fmt.Errorf("pull %s: %w", svc.Image, err)
	}

	id := fmt.Sprintf("%s-%s-%d", projectName, name, index)

	var envVars []string
	for k, v := range svc.Environment {
		envVars = append(envVars, k+"="+v)
	}

	ports, _ := parseComposePorts(svc.Ports)
	mounts := parseComposeVolumes(svc.Volumes)

	netMode := ""
	if len(svc.Networks) > 0 {
		netMode = svc.Networks[0]
	}

	cfg := runtime.ContainerConfig{
		ID:          id,
		Image:       svc.Image,
		Command:     svc.Command,
		Env:         envVars,
		Ports:       ports,
		Mounts:      mounts,
		NetworkMode: netMode,
	}
	if svc.Restart != "" {
		if policy, err := runtime.ParseRestartPolicy(svc.Restart); err == nil {
			cfg.RestartPolicy = policy
		}
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

func servicePrefix(project, service string) string {
	return project + "-" + service + "-"
}

// serviceIDs returns existing container IDs for a service, sorted.
func serviceIDs(project, service string) []string {
	entries, err := os.ReadDir("/run/thrive/containers")
	if err != nil {
		return []string{containerName(project, service)}
	}
	prefix := servicePrefix(project, service)
	var ids []string
	for _, e := range entries {
		if e.IsDir() && len(e.Name()) > len(prefix) && e.Name()[:len(prefix)] == prefix {
			ids = append(ids, e.Name())
		}
	}
	sortStrings(ids)
	if len(ids) == 0 {
		return []string{containerName(project, service)}
	}
	return ids
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
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
		src := spec[:idx]
		if volume.IsNamedVolume(src) {
			if vol, err := volume.Ensure(src); err == nil {
				src = vol.Path
			}
		}
		mounts = append(mounts, runtime.Mount{
			Source:      src,
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
