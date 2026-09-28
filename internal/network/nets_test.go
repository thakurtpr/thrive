package network

import (
	"strings"
	"testing"
)

func testNetOverride(t *testing.T) {
	t.Helper()
	networkDirOverride = t.TempDir()
	t.Cleanup(func() { networkDirOverride = "" })
}

// skipIfNoNetAdmin skips bridge-dependent tests where the runner lacks
// CAP_NET_ADMIN (e.g. GitHub Actions): EnsureBridge fails with
// "RTNETLINK answers: Operation not permitted". Validation-only assertions
// must not use this — they run before bridge setup and stay meaningful.
func skipIfNoNetAdmin(t *testing.T, err error) {
	t.Helper()
	if err != nil && strings.Contains(err.Error(), "Operation not permitted") {
		t.Skip("skipping: no CAP_NET_ADMIN for bridge setup")
	}
}

// TestCreateInspectList verifies network creation and discovery.
// Bridge/NAT setup is skipped off-Linux; metadata paths are exercised.
func TestCreateInspectList(t *testing.T) {
	testNetOverride(t)
	nw, err := CreateNetwork("frontend", "", "")
	skipIfNoNetAdmin(t, err)
	if err != nil {
		t.Fatalf("CreateNetwork: %v", err)
	}
	if nw.Driver != "bridge" {
		t.Errorf("Driver: got %q, want bridge", nw.Driver)
	}
	if nw.Subnet == Subnet {
		t.Errorf("Subnet: custom network reused default %q", nw.Subnet)
	}
	if nw.Bridge == BridgeName || nw.Bridge == "" {
		t.Errorf("Bridge: got %q, want custom bridge", nw.Bridge)
	}

	got, err := InspectNetwork("frontend")
	if err != nil {
		t.Fatalf("InspectNetwork: %v", err)
	}
	if got.Subnet != nw.Subnet {
		t.Errorf("Subnet: got %q, want %q", got.Subnet, nw.Subnet)
	}

	nets, err := ListNetworks()
	if err != nil {
		t.Fatalf("ListNetworks: %v", err)
	}
	if len(nets) != 2 { // bridge + frontend
		t.Errorf("ListNetworks: got %d, want 2", len(nets))
	}
}

// TestCreate_Validation verifies name/driver/reserved handling.
func TestCreate_Validation(t *testing.T) {
	testNetOverride(t)
	for _, bad := range []string{"", "has space", "bridge", "host", "none"} {
		if _, err := CreateNetwork(bad, "", ""); err == nil {
			t.Errorf("CreateNetwork(%q): expected error, got nil", bad)
		}
	}
	if _, err := CreateNetwork("ok", "overlay", ""); err == nil {
		t.Error("CreateNetwork overlay: expected error, got nil")
	}
	if _, err := CreateNetwork("dup", "", ""); err != nil {
		skipIfNoNetAdmin(t, err)
		t.Fatalf("CreateNetwork: %v", err)
	}
	if _, err := CreateNetwork("dup", "", ""); err == nil {
		t.Error("CreateNetwork duplicate: expected error, got nil")
	}
}

// TestConnectDisconnect verifies attachment records.
func TestConnectDisconnect(t *testing.T) {
	testNetOverride(t)
	if _, err := CreateNetwork("app", "", ""); err != nil {
		skipIfNoNetAdmin(t, err)
		t.Fatal(err)
	}
	if err := ConnectNetwork("ctr1", "app"); err != nil {
		t.Fatalf("ConnectNetwork: %v", err)
	}
	nw, _ := InspectNetwork("app")
	if len(nw.Containers) != 1 {
		t.Errorf("Containers: got %d, want 1", len(nw.Containers))
	}
	if err := DisconnectNetwork("ctr1", "app"); err != nil {
		t.Fatalf("DisconnectNetwork: %v", err)
	}
	nw, _ = InspectNetwork("app")
	if len(nw.Containers) != 0 {
		t.Errorf("Containers after disconnect: got %d, want 0", len(nw.Containers))
	}
	if err := ConnectNetwork("ctr1", "missing"); err == nil {
		t.Error("ConnectNetwork missing: expected error, got nil")
	}
}

// TestRemove_Guards verifies reserved/refuse behaviour.
func TestRemove_Guards(t *testing.T) {
	testNetOverride(t)
	if err := RemoveNetwork("bridge", true); err == nil {
		t.Error("RemoveNetwork bridge: expected error, got nil")
	}
	if _, err := CreateNetwork("busy", "", ""); err != nil {
		skipIfNoNetAdmin(t, err)
		t.Fatal(err)
	}
	if err := ConnectNetwork("ctr9", "busy"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveNetwork("busy", false); err == nil {
		t.Error("RemoveNetwork attached: expected error, got nil")
	}
	if err := RemoveNetwork("busy", true); err != nil {
		t.Errorf("RemoveNetwork --force: %v", err)
	}
}

// TestPrune verifies only unused custom networks are removed.
func TestPrune(t *testing.T) {
	testNetOverride(t)
	if _, err := CreateNetwork("idle", "", ""); err != nil {
		skipIfNoNetAdmin(t, err)
		t.Fatal(err)
	}
	if _, err := CreateNetwork("used", "", ""); err != nil {
		skipIfNoNetAdmin(t, err)
		t.Fatal(err)
	}
	if err := ConnectNetwork("ctr1", "used"); err != nil {
		t.Fatal(err)
	}
	removed, err := PruneNetworks()
	if err != nil {
		t.Fatalf("PruneNetworks: %v", err)
	}
	if len(removed) != 1 || removed[0] != "idle" {
		t.Errorf("PruneNetworks: got %v, want [idle]", removed)
	}
}

// TestAllocateOn verifies per-network IPAM uniqueness.
func TestAllocateOn(t *testing.T) {
	testNetOverride(t)
	nw, err := CreateNetwork("ipamnet", "", "172.25.0.0/16")
	skipIfNoNetAdmin(t, err)
	if err != nil {
		t.Fatalf("CreateNetwork: %v", err)
	}
	ip1, err := AllocateOn(nw, "c1")
	if err != nil {
		t.Fatalf("AllocateOn: %v", err)
	}
	ip2, err := AllocateOn(nw, "c2")
	if err != nil {
		t.Fatalf("AllocateOn: %v", err)
	}
	if ip1 == ip2 {
		t.Errorf("AllocateOn: duplicate IP %q", ip1)
	}
	if len(ip1) < 8 || ip1[:7] != "172.25." {
		t.Errorf("AllocateOn: IP %q not in 172.25.0.0/16", ip1)
	}
	ReleaseOn(nw, "c1")
}

// TestValidNetworkName verifies name rules.
func TestValidNetworkName(t *testing.T) {
	for name, want := range map[string]bool{
		"frontend": true, "net-1": true, "a.b_c": true, "": false,
		"has space": false, "bridge-x": true, "a/b": false,
	} {
		if got := validNetworkName(name); got != want {
			t.Errorf("validNetworkName(%q): got %v, want %v", name, got, want)
		}
	}
}

// TestBridgeFor verifies stable, bridge-safe names.
func TestBridgeFor(t *testing.T) {
	a, b := bridgeFor("frontend"), bridgeFor("frontend")
	if a != b || len(a) > 15 || a == BridgeName {
		t.Errorf("bridgeFor: got %q", a)
	}
	if bridgeFor("frontend") == bridgeFor("backend") {
		t.Error("bridgeFor: collision between different names")
	}
}

// TestSubnetBase verifies CIDR base extraction.
func TestSubnetBase(t *testing.T) {
	if got := subnetBase("172.25.0.0/16"); got != "172.25" {
		t.Errorf("subnetBase: got %q", got)
	}
}

// TestInspectNetworkBuiltin verifies the default bridge resolves.
func TestInspectNetworkBuiltin(t *testing.T) {
	nw, err := InspectNetwork("bridge")
	if err != nil {
		t.Fatalf("InspectNetwork(bridge): %v", err)
	}
	if nw.Bridge != BridgeName || nw.Subnet != Subnet {
		t.Errorf("builtin: got %+v", nw)
	}
}
