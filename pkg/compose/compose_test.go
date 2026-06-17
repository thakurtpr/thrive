//go:build linux

package compose

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	content := "version: \"3\"\nservices:\n  web:\n    image: nginx\n    ports:\n      - \"8080:80\"\n  db:\n    image: postgres\n    environment:\n      POSTGRES_PASSWORD: secret\n"
	path := filepath.Join(dir, "docker-compose.yml")
	os.WriteFile(path, []byte(content), 0644)

	cf, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cf.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(cf.Services))
	}
	if cf.Services["web"].Image != "nginx" {
		t.Errorf("expected web image nginx, got %s", cf.Services["web"].Image)
	}
}

func TestLoad_NoServices(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "docker-compose.yml")
	os.WriteFile(path, []byte("version: \"3\"\n"), 0644)

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for file with no services")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	if _, err := Load("/nonexistent/path/docker-compose.yml"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestStartOrder_NoDeps(t *testing.T) {
	cf := &ComposeFile{
		Services: map[string]*ServiceDef{
			"web": {Image: "nginx"},
			"db":  {Image: "postgres"},
		},
	}
	order, err := startOrder(cf)
	if err != nil {
		t.Fatalf("startOrder: %v", err)
	}
	if len(order) != 2 {
		t.Errorf("expected 2 services, got %d", len(order))
	}
}

func TestStartOrder_WithDeps(t *testing.T) {
	cf := &ComposeFile{
		Services: map[string]*ServiceDef{
			"web": {Image: "nginx", DependsOn: []string{"db"}},
			"db":  {Image: "postgres"},
		},
	}
	order, err := startOrder(cf)
	if err != nil {
		t.Fatalf("startOrder: %v", err)
	}
	dbIdx, webIdx := -1, -1
	for i, name := range order {
		switch name {
		case "db":
			dbIdx = i
		case "web":
			webIdx = i
		}
	}
	if dbIdx > webIdx {
		t.Errorf("db (%d) should come before web (%d) in start order", dbIdx, webIdx)
	}
}

func TestStartOrder_CircularDep(t *testing.T) {
	cf := &ComposeFile{
		Services: map[string]*ServiceDef{
			"a": {Image: "x", DependsOn: []string{"b"}},
			"b": {Image: "y", DependsOn: []string{"a"}},
		},
	}
	if _, err := startOrder(cf); err == nil {
		t.Fatal("expected circular dependency error")
	}
}

func TestContainerName(t *testing.T) {
	if got := containerName("myapp", "web"); got != "myapp-web-1" {
		t.Errorf("containerName = %q, want myapp-web-1", got)
	}
}

func TestParseComposePorts(t *testing.T) {
	ports, err := parseComposePorts([]string{"8080:80", "443:443/tcp"})
	if err != nil {
		t.Fatalf("parseComposePorts: %v", err)
	}
	if len(ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(ports))
	}
	if ports[0].HostPort != 8080 || ports[0].ContainerPort != 80 {
		t.Errorf("port[0]: host=%d container=%d", ports[0].HostPort, ports[0].ContainerPort)
	}
}

func TestParseComposeVolumes(t *testing.T) {
	mounts := parseComposeVolumes([]string{"/data:/var/data", "/tmp"})
	if len(mounts) != 2 {
		t.Fatalf("expected 2 mounts, got %d", len(mounts))
	}
	if mounts[0].Source != "/data" || mounts[0].Destination != "/var/data" {
		t.Errorf("mount[0]: src=%s dst=%s", mounts[0].Source, mounts[0].Destination)
	}
}
