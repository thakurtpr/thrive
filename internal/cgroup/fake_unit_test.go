package cgroup_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/thakurprasadrout/thrive/internal/cgroup"
)

type fakeCgroupManager struct {
	mu       sync.Mutex
	applied  map[string]cgroup.Limits
	applyErr error
}

func newFakeCgroupManager() *fakeCgroupManager {
	return &fakeCgroupManager{applied: make(map[string]cgroup.Limits)}
}

func (m *fakeCgroupManager) Apply(id string, limits cgroup.Limits) error {
	if m.applyErr != nil {
		return m.applyErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.applied[id] = limits
	return nil
}

func (m *fakeCgroupManager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.applied, id)
	return nil
}

func TestFakeCgroupApplyAndRemove(t *testing.T) {
	m := newFakeCgroupManager()
	limits := cgroup.Limits{MemoryBytes: 128 << 20, CPUQuota: 50000, PIDsMax: 100}
	if err := m.Apply("ctr-1", limits); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	m.mu.Lock()
	got, ok := m.applied["ctr-1"]
	m.mu.Unlock()
	if !ok {
		t.Fatal("limits not stored")
	}
	if got.MemoryBytes != 128<<20 {
		t.Errorf("expected 128 MiB, got %d", got.MemoryBytes)
	}
	if err := m.Remove("ctr-1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	m.mu.Lock()
	_, exists := m.applied["ctr-1"]
	m.mu.Unlock()
	if exists {
		t.Error("limits should be removed after Remove")
	}
}

func TestFakeCgroupApplyError(t *testing.T) {
	m := newFakeCgroupManager()
	m.applyErr = fmt.Errorf("cgroup v2 not mounted")
	if err := m.Apply("ctr-2", cgroup.Limits{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestFakeCgroupZeroLimitsAllowed(t *testing.T) {
	m := newFakeCgroupManager()
	if err := m.Apply("ctr-3", cgroup.Limits{}); err != nil {
		t.Fatalf("zero limits (unlimited) should be accepted: %v", err)
	}
}
