//go:build !linux

package network_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/thakurprasadrout/thrive/internal/network"
)

type fakeNetworker struct {
	mu       sync.Mutex
	active   map[string]network.NetConfig
	setupErr error
}

func newFakeNetworker() *fakeNetworker {
	return &fakeNetworker{active: make(map[string]network.NetConfig)}
}

func (n *fakeNetworker) Setup(id string, cfg network.NetConfig) error {
	if n.setupErr != nil {
		return n.setupErr
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.active[id] = cfg
	return nil
}

func (n *fakeNetworker) Teardown(id string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.active, id)
	return nil
}

// Ensure fakeNetworker implements the Networker interface at compile time.
var _ network.Networker = (*fakeNetworker)(nil)

func TestFakeNetworkerSetupTeardown(t *testing.T) {
	net := newFakeNetworker()
	cfg := network.NetConfig{
		BridgeName:  "thrive0",
		HostIP:      "10.88.0.1",
		ContainerIP: "10.88.0.2",
		PortMappings: []network.PortMapping{
			{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
		},
	}
	if err := net.Setup("ctr-1", cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	net.mu.Lock()
	got, ok := net.active["ctr-1"]
	net.mu.Unlock()
	if !ok {
		t.Fatal("net config not stored")
	}
	if got.BridgeName != "thrive0" {
		t.Errorf("expected thrive0, got %s", got.BridgeName)
	}
	if err := net.Teardown("ctr-1"); err != nil {
		t.Fatalf("Teardown: %v", err)
	}
	net.mu.Lock()
	_, exists := net.active["ctr-1"]
	net.mu.Unlock()
	if exists {
		t.Error("network should be torn down after Teardown")
	}
}

func TestFakeNetworkerSetupError(t *testing.T) {
	net := newFakeNetworker()
	net.setupErr = fmt.Errorf("bridge already exists")
	if err := net.Setup("ctr-2", network.NetConfig{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestPortMappingFields(t *testing.T) {
	pm := network.PortMapping{HostPort: 3000, ContainerPort: 3000, Protocol: "tcp"}
	if pm.HostPort != 3000 {
		t.Errorf("unexpected host port %d", pm.HostPort)
	}
	if pm.Protocol != "tcp" {
		t.Errorf("expected tcp, got %s", pm.Protocol)
	}
}

func TestNetConfigFields(t *testing.T) {
	cfg := network.NetConfig{
		BridgeName:   "br0",
		HostIP:       "192.168.1.1",
		ContainerIP:  "192.168.1.2",
		PortMappings: nil,
	}
	if cfg.BridgeName != "br0" {
		t.Errorf("unexpected bridge name %s", cfg.BridgeName)
	}
	if cfg.HostIP != "192.168.1.1" {
		t.Errorf("unexpected host IP %s", cfg.HostIP)
	}
}
