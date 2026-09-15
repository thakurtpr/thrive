//go:build linux
// +build linux

package swarm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/thakurprasadrout/thrive/internal/image"
	"github.com/thakurprasadrout/thrive/internal/runtime"
)

// ServiceSpec describes the desired state of a service.
type ServiceSpec struct {
	Image       string                `json:"image"`
	Command     []string              `json:"command,omitempty"`
	Env         []string              `json:"env,omitempty"`
	Replicas    int                   `json:"replicas"`
	Ports       []runtime.PortMapping `json:"ports,omitempty"`
	Mounts      []runtime.Mount       `json:"mounts,omitempty"`
	NetworkMode string                `json:"networkMode,omitempty"`
	Parallelism int                   `json:"parallelism,omitempty"`
	DelaySecs   int                   `json:"delaySecs,omitempty"`
}

// Service is a persisted replicated service.
type Service struct {
	Name     string       `json:"name"`
	Spec     ServiceSpec  `json:"spec"`
	Previous *ServiceSpec `json:"previous,omitempty"`
	Created  time.Time    `json:"created"`
	Updated  time.Time    `json:"updated"`
}

func servicesDir() string { return filepath.Join(rootDir(), "services") }

func servicePath(name string) string { return filepath.Join(servicesDir(), name+".json") }

func validServiceName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			c == '_' || c == '.' || c == '-') {
			return false
		}
	}
	return true
}

func containerPrefix(name string) string { return "swarm-" + name + "-" }

// CreateService creates and reconciles a service.
func CreateService(ctx context.Context, name string, spec ServiceSpec) (*Service, error) {
	if err := RequireInitialised(); err != nil {
		return nil, err
	}
	if !validServiceName(name) {
		return nil, fmt.Errorf("swarm: invalid service name %q", name)
	}
	if _, err := os.Stat(servicePath(name)); err == nil {
		return nil, fmt.Errorf("swarm: service %s already exists", name)
	}
	if spec.Image == "" {
		return nil, fmt.Errorf("swarm: image required")
	}
	if spec.Replicas < 1 {
		spec.Replicas = 1
	}
	if spec.Parallelism < 1 {
		spec.Parallelism = 1
	}
	svc := &Service{Name: name, Spec: spec, Created: time.Now().UTC(), Updated: time.Now().UTC()}
	if err := writeService(svc); err != nil {
		return nil, err
	}
	if err := Reconcile(ctx, name); err != nil {
		return nil, err
	}
	return svc, nil
}

// InspectService returns a service by name.
func InspectService(name string) (*Service, error) {
	data, err := os.ReadFile(servicePath(name))
	if err != nil {
		return nil, fmt.Errorf("swarm: service %s not found: %w", name, err)
	}
	var svc Service
	if err := json.Unmarshal(data, &svc); err != nil {
		return nil, fmt.Errorf("swarm: parse service: %w", err)
	}
	return &svc, nil
}

// ListServices returns all services.
func ListServices() ([]*Service, error) {
	entries, err := os.ReadDir(servicesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("swarm: list: %w", err)
	}
	var out []*Service
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) > 5 && name[len(name)-5:] == ".json" {
			if svc, err := InspectService(name[:len(name)-5]); err == nil {
				out = append(out, svc)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// RemoveService deletes a service and its containers.
func RemoveService(ctx context.Context, name string) error {
	svc, err := InspectService(name)
	if err != nil {
		return err
	}
	_ = svc
	for _, id := range serviceContainers(name) {
		_ = runtime.Kill(ctx, id, 9)
		_ = runtime.Delete(ctx, id)
	}
	if err := os.Remove(servicePath(name)); err != nil {
		return fmt.Errorf("swarm: remove: %w", err)
	}
	return nil
}

// ScaleService sets the replica count and reconciles.
func ScaleService(ctx context.Context, name string, replicas int) error {
	if replicas < 0 {
		return fmt.Errorf("swarm: replicas must be >= 0")
	}
	svc, err := InspectService(name)
	if err != nil {
		return err
	}
	svc.Spec.Replicas = replicas
	svc.Updated = time.Now().UTC()
	if err := writeService(svc); err != nil {
		return err
	}
	return Reconcile(ctx, name)
}

// UpdateService performs a rolling update to a new image.
func UpdateService(ctx context.Context, name, newImage string) error {
	svc, err := InspectService(name)
	if err != nil {
		return err
	}
	if newImage == "" {
		return fmt.Errorf("swarm: update requires --image")
	}
	prev := svc.Spec
	svc.Previous = &prev
	svc.Spec.Image = newImage
	svc.Updated = time.Now().UTC()
	if err := writeService(svc); err != nil {
		return err
	}
	return rollingReplace(ctx, svc)
}

// RollbackService restores the previous spec with a rolling replace.
func RollbackService(ctx context.Context, name string) error {
	svc, err := InspectService(name)
	if err != nil {
		return err
	}
	if svc.Previous == nil {
		return fmt.Errorf("swarm: service %s has no previous spec", name)
	}
	cur := svc.Spec
	svc.Spec = *svc.Previous
	svc.Previous = &cur
	svc.Updated = time.Now().UTC()
	if err := writeService(svc); err != nil {
		return err
	}
	return rollingReplace(ctx, svc)
}

// Reconcile converges running containers to the desired replica count.
func Reconcile(ctx context.Context, name string) error {
	svc, err := InspectService(name)
	if err != nil {
		return err
	}
	existing := serviceContainers(name)
	// Remove extras (highest index first).
	for len(existing) > svc.Spec.Replicas {
		id := existing[len(existing)-1]
		_ = runtime.Kill(ctx, id, 9)
		if err := runtime.Delete(ctx, id); err != nil {
			return fmt.Errorf("swarm: remove %s: %w", id, err)
		}
		existing = existing[:len(existing)-1]
	}
	// Start missing replicas at the lowest free indices.
	used := map[int]bool{}
	for _, id := range existing {
		if idx := containerIndex(name, id); idx > 0 {
			used[idx] = true
		}
	}
	for i := 1; len(serviceContainers(name)) < svc.Spec.Replicas; i++ {
		if used[i] {
			continue
		}
		if err := startReplica(ctx, svc, i); err != nil {
			return err
		}
		used[i] = true
	}
	return nil
}

// rollingReplace swaps replicas to the current spec in batches.
func rollingReplace(ctx context.Context, svc *Service) error {
	if _, err := image.Pull(ctx, svc.Spec.Image, image.PullOptions{}); err != nil {
		return fmt.Errorf("swarm: pull %s: %w", svc.Spec.Image, err)
	}
	parallelism := svc.Spec.Parallelism
	if parallelism < 1 {
		parallelism = 1
	}
	ids := serviceContainers(svc.Name)
	for start := 0; start < len(ids); start += parallelism {
		end := start + parallelism
		if end > len(ids) {
			end = len(ids)
		}
		for _, id := range ids[start:end] {
			idx := containerIndex(svc.Name, id)
			if idx <= 0 {
				idx = start + 1
			}
			_ = runtime.Kill(ctx, id, 9)
			_ = runtime.Delete(ctx, id)
			if err := startReplica(ctx, svc, idx); err != nil {
				return fmt.Errorf("swarm: replace %s: %w", id, err)
			}
		}
		if svc.Spec.DelaySecs > 0 && end < len(ids) {
			time.Sleep(time.Duration(svc.Spec.DelaySecs) * time.Second)
		}
	}
	// Scale from zero: no existing containers to replace.
	if len(ids) == 0 {
		return Reconcile(ctx, svc.Name)
	}
	return nil
}

func startReplica(ctx context.Context, svc *Service, index int) error {
	id := fmt.Sprintf("%s%s-%d", "swarm-", svc.Name, index)
	cfg := runtime.ContainerConfig{
		ID:          id,
		Image:       svc.Spec.Image,
		Command:     svc.Spec.Command,
		Env:         svc.Spec.Env,
		Ports:       svc.Spec.Ports,
		Mounts:      svc.Spec.Mounts,
		NetworkMode: svc.Spec.NetworkMode,
	}
	if _, err := runtime.Create(ctx, cfg); err != nil {
		return fmt.Errorf("create %s: %w", id, err)
	}
	if _, err := runtime.Start(ctx, id); err != nil {
		_ = runtime.Delete(ctx, id)
		return fmt.Errorf("start %s: %w", id, err)
	}
	return nil
}

// serviceContainers returns existing container IDs for a service, sorted.
func serviceContainers(name string) []string {
	entries, err := os.ReadDir("/run/thrive/containers")
	if err != nil {
		return nil
	}
	prefix := containerPrefix(name)
	var ids []string
	for _, e := range entries {
		if e.IsDir() && len(e.Name()) > len(prefix) && e.Name()[:len(prefix)] == prefix {
			ids = append(ids, e.Name())
		}
	}
	sort.Strings(ids)
	return ids
}

func containerIndex(name, id string) int {
	n, err := strconv.Atoi(id[len(containerPrefix(name)):])
	if err != nil {
		return 0
	}
	return n
}

func writeService(svc *Service) error {
	if err := os.MkdirAll(servicesDir(), 0755); err != nil {
		return fmt.Errorf("swarm: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(svc, "", "  ")
	if err != nil {
		return fmt.Errorf("swarm: marshal: %w", err)
	}
	if err := os.WriteFile(servicePath(svc.Name), data, 0644); err != nil {
		return fmt.Errorf("swarm: write: %w", err)
	}
	return nil
}
