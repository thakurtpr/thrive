package runtime_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/thakurprasadrout/thrive/internal/runtime"
)

type fakeRunner struct {
	mu         sync.Mutex
	containers map[string]runtime.ContainerState
	startErr   error
	stopErr    error
	nextID     int
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{containers: make(map[string]runtime.ContainerState)}
}

func (r *fakeRunner) Start(cfg runtime.ContainerConfig) (string, error) {
	if r.startErr != nil {
		return "", r.startErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	id := fmt.Sprintf("ctr-%d", r.nextID)
	r.containers[id] = runtime.ContainerState{ID: id, Status: "running", PID: 12345 + r.nextID}
	return id, nil
}

func (r *fakeRunner) Stop(id string) error {
	if r.stopErr != nil {
		return r.stopErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.containers[id]
	if !ok {
		return fmt.Errorf("container not found: %s", id)
	}
	s.Status = "stopped"
	s.PID = 0
	r.containers[id] = s
	return nil
}

func (r *fakeRunner) Status(id string) (runtime.ContainerState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.containers[id]
	if !ok {
		return runtime.ContainerState{}, fmt.Errorf("container not found: %s", id)
	}
	return s, nil
}

func (r *fakeRunner) Logs(_ string) ([]byte, error) {
	return []byte("fake log line\n"), nil
}

func TestFakeRunnerStartStop(t *testing.T) {
	r := newFakeRunner()
	id, err := r.Start(runtime.ContainerConfig{Image: "alpine:3.19"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty container ID")
	}
	st, err := r.Status(id)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Status != "running" {
		t.Errorf("expected running, got %s", st.Status)
	}
	if err := r.Stop(id); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	st, _ = r.Status(id)
	if st.Status != "stopped" {
		t.Errorf("expected stopped, got %s", st.Status)
	}
}

func TestFakeRunnerStartError(t *testing.T) {
	r := newFakeRunner()
	r.startErr = fmt.Errorf("no rootfs")
	if _, err := r.Start(runtime.ContainerConfig{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestFakeRunnerStopUnknown(t *testing.T) {
	r := newFakeRunner()
	if err := r.Stop("ghost-id"); err == nil {
		t.Fatal("expected error for unknown container")
	}
}

func TestFakeRunnerLogs(t *testing.T) {
	r := newFakeRunner()
	id, _ := r.Start(runtime.ContainerConfig{Image: "alpine:3.19"})
	logs, err := r.Logs(id)
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}
	if len(logs) == 0 {
		t.Error("expected non-empty logs")
	}
}
