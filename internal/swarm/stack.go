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
	"time"

	"github.com/thakurprasadrout/thrive/pkg/compose"
)

// Stack is a deployed compose application tracked by swarm.
type Stack struct {
	Name     string    `json:"name"`
	Services []string  `json:"services"`
	File     string    `json:"file"`
	Deployed time.Time `json:"deployed"`
}

func stacksDir() string { return filepath.Join(rootDir(), "stacks") }

func stackPath(name string) string { return filepath.Join(stacksDir(), name+".json") }

// DeployStack creates one service per compose service (namespaced by stack).
func DeployStack(ctx context.Context, stackName string, cf *compose.ComposeFile) (*Stack, error) {
	if err := RequireInitialised(); err != nil {
		return nil, err
	}
	if !validServiceName(stackName) {
		return nil, fmt.Errorf("swarm: invalid stack name %q", stackName)
	}
	if _, err := os.Stat(stackPath(stackName)); err == nil {
		return nil, fmt.Errorf("swarm: stack %s already exists (rm first)", stackName)
	}
	st := &Stack{Name: stackName, Deployed: time.Now().UTC()}
	for svcName, def := range cf.Services {
		name := stackName + "_" + svcName
		spec := ServiceSpec{
			Image:    def.Image,
			Command:  def.Command,
			Replicas: 1,
		}
		for k, v := range def.Environment {
			spec.Env = append(spec.Env, k+"="+v)
		}
		if len(def.Networks) > 0 {
			spec.NetworkMode = def.Networks[0]
		}
		if _, err := CreateService(ctx, name, spec); err != nil {
			return nil, fmt.Errorf("swarm: deploy service %s: %w", name, err)
		}
		st.Services = append(st.Services, name)
	}
	sort.Strings(st.Services)
	if err := writeStack(st); err != nil {
		return nil, err
	}
	return st, nil
}

// InspectStack returns a stack by name.
func InspectStack(name string) (*Stack, error) {
	data, err := os.ReadFile(stackPath(name))
	if err != nil {
		return nil, fmt.Errorf("swarm: stack %s not found: %w", name, err)
	}
	var st Stack
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("swarm: parse stack %s: %w", name, err)
	}
	return &st, nil
}

// ListStacks returns all stacks.
func ListStacks() ([]*Stack, error) {
	entries, err := os.ReadDir(stacksDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("swarm: list stacks: %w", err)
	}
	var out []*Stack
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) > 5 && name[len(name)-5:] == ".json" {
			if st, err := InspectStack(name[:len(name)-5]); err == nil {
				out = append(out, st)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// RemoveStack deletes a stack and all its services.
func RemoveStack(ctx context.Context, name string) error {
	st, err := InspectStack(name)
	if err != nil {
		return err
	}
	for _, svc := range st.Services {
		if err := RemoveService(ctx, svc); err != nil {
			return fmt.Errorf("swarm: remove service %s: %w", svc, err)
		}
	}
	if err := os.Remove(stackPath(name)); err != nil {
		return fmt.Errorf("swarm: remove stack: %w", err)
	}
	return nil
}

func writeStack(st *Stack) error {
	if err := os.MkdirAll(stacksDir(), 0755); err != nil {
		return fmt.Errorf("swarm: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("swarm: marshal: %w", err)
	}
	if err := os.WriteFile(stackPath(st.Name), data, 0644); err != nil {
		return fmt.Errorf("swarm: write: %w", err)
	}
	return nil
}
