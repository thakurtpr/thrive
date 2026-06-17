//go:build linux

package network

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteResolvConf(t *testing.T) {
	rootfs := t.TempDir()
	if err := WriteResolvConf(rootfs); err != nil {
		t.Fatalf("WriteResolvConf: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(rootfs, "etc", "resolv.conf"))
	if err != nil {
		t.Fatalf("ReadFile resolv.conf: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("resolv.conf is empty")
	}
}

func TestWriteResolvConf_EmptyPath(t *testing.T) {
	if err := WriteResolvConf(""); err != nil {
		t.Fatalf("WriteResolvConf with empty path: %v", err)
	}
}

func TestBridgeExists_DoesNotPanic(t *testing.T) {
	_ = bridgeExists()
}

func TestAllocateReleaseIP(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("requires root")
	}
	ip1, err := allocateIP("testctr-a")
	if err != nil {
		t.Fatalf("allocateIP: %v", err)
	}
	ip2, err := allocateIP("testctr-b")
	if err != nil {
		t.Fatalf("allocateIP: %v", err)
	}
	if ip1 == ip2 {
		t.Errorf("expected unique IPs, got both %s", ip1)
	}
	releaseIP("testctr-a")
	releaseIP("testctr-b")
	if _, statErr := os.Stat(filepath.Join("/run/thrive/network", "ip-testctr-a")); !os.IsNotExist(statErr) {
		t.Error("ip file not removed after release")
	}
}

func TestPublishPort_InvalidHostPort(t *testing.T) {
	for _, hp := range []int{0, -1, 65536} {
		if err := PublishPort(hp, 80, "172.18.0.2"); err == nil {
			t.Errorf("PublishPort(%d,...): expected error", hp)
		}
	}
}

func TestPublishPort_InvalidContainerPort(t *testing.T) {
	if err := PublishPort(8080, 0, "172.18.0.2"); err == nil {
		t.Error("expected error for container port 0")
	}
}

func TestPublishPort_EmptyContainerIP(t *testing.T) {
	if err := PublishPort(8080, 80, ""); err == nil {
		t.Error("expected error for empty containerIP")
	}
}

func TestPublishPort_ValidArgs(t *testing.T) {
	err := PublishPort(18080, 18080, "172.18.99.2")
	if err != nil {
		if strings.Contains(err.Error(), "out of range") || strings.Contains(err.Error(), "must not be empty") {
			t.Errorf("validation error for valid args: %v", err)
		}
		return
	}
	UnpublishPort(18080, 18080)
}

func TestUnpublishPort_DoesNotPanic(t *testing.T) {
	UnpublishPort(19999, 19999)
}

func TestValidatePort(t *testing.T) {
	cases := []struct {
		port    int
		wantErr bool
	}{
		{1, false}, {80, false}, {65535, false},
		{0, true}, {-1, true}, {65536, true}, {100000, true},
	}
	for _, tc := range cases {
		err := validatePort(tc.port)
		if tc.wantErr && err == nil {
			t.Errorf("validatePort(%d): expected error", tc.port)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("validatePort(%d): unexpected error: %v", tc.port, err)
		}
	}
}

func TestCNISetup_FallbackWhenBinsAbsent(t *testing.T) {
	_, err := CNISetup("test-ctr", "/proc/1/ns/net", nil)
	if err == nil {
		t.Skip("CNI binaries present; skipping fallback test")
	}
	if !strings.Contains(err.Error(), "CNI plugins not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCNITeardown_NoopWhenBinsAbsent(t *testing.T) {
	if err := CNITeardown("test-ctr", "/proc/1/ns/net"); err != nil {
		t.Errorf("expected nil when bins absent, got: %v", err)
	}
}

func TestSetupContainer_FailsWithoutRoot(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root")
	}
	if _, err := SetupContainer("test-sc", 1, "", nil); err == nil {
		t.Error("expected error without root")
	}
}
