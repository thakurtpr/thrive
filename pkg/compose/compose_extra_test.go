//go:build linux
// +build linux

package compose

import (
	"context"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestFilterServices verifies target filtering.
func TestFilterServices(t *testing.T) {
	cf := &ComposeFile{Services: map[string]*ServiceDef{
		"web": {}, "db": {},
	}}
	if got := filterServices(cf, nil); len(got) != 2 {
		t.Errorf("filterServices(nil): got %v", got)
	}
	got := filterServices(cf, []string{"web", "missing"})
	if len(got) != 1 || got[0] != "web" {
		t.Errorf("filterServices: got %v", got)
	}
}

// TestServiceIDs_MissingDir verifies the default ID fallback.
func TestServiceIDs_MissingDir(t *testing.T) {
	ids := serviceIDs("no-proj-xyz", "web")
	if len(ids) != 1 || ids[0] != "no-proj-xyz-web-1" {
		t.Errorf("serviceIDs: got %v", ids)
	}
}

// TestConfig_Roundtrip verifies config rendering contains services.
func TestConfig_Roundtrip(t *testing.T) {
	cf := &ComposeFile{
		Version:  "3",
		Services: map[string]*ServiceDef{"web": {Image: "nginx"}},
	}
	out, err := Config(cf)
	if err != nil {
		t.Fatalf("Config: %v", err)
	}
	if !strings.Contains(out, "nginx") || !strings.Contains(out, "web") {
		t.Errorf("Config: missing content in %q", out)
	}
}

// TestBuild_NoBuildSections verifies Build skips imageless services cleanly.
func TestBuild_NoBuildSections(t *testing.T) {
	cf := &ComposeFile{Services: map[string]*ServiceDef{
		"web": {Image: "nginx"},
	}}
	if err := Build(context.Background(), cf, t.TempDir(), nil); err != nil {
		t.Errorf("Build: %v", err)
	}
	if err := Build(context.Background(), cf, t.TempDir(), []string{"missing"}); err != nil {
		t.Errorf("Build unknown target: %v", err)
	}
}

// TestStopKillRm_Missing verifies no-ops on absent containers.
func TestStopKillRm_Missing(t *testing.T) {
	ctx := context.Background()
	cf := &ComposeFile{Services: map[string]*ServiceDef{
		"web": {Image: "nginx"},
	}}
	// serviceIDs falls back to proj-web-1 which does not exist: all skip.
	if err := Stop(ctx, cf, "no-proj-xyz", nil, 0); err != nil {
		t.Errorf("Stop: %v", err)
	}
	if err := Kill(ctx, cf, "no-proj-xyz", nil, 9); err != nil {
		t.Errorf("Kill: %v", err)
	}
	if err := Rm(ctx, cf, "no-proj-xyz", nil, false); err != nil {
		t.Errorf("Rm: %v", err)
	}
}

// TestStart_Missing verifies Start errors clearly for absent containers.
func TestStart_Missing(t *testing.T) {
	cf := &ComposeFile{Services: map[string]*ServiceDef{
		"web": {Image: "nginx"},
	}}
	if err := Start(context.Background(), cf, "no-proj-xyz", nil); err == nil {
		t.Error("Start: expected error for missing container, got nil")
	}
}

// TestBuildDefUnmarshal verifies string and map build forms.
func TestBuildDefUnmarshal(t *testing.T) {
	for _, doc := range []string{
		"services:\n  web:\n    build: ./app\n",
		"services:\n  web:\n    build:\n      context: ./app\n      dockerfile: Alt\n      args:\n        V: \"1\"\n",
	} {
		var cf ComposeFile
		if err := yaml.Unmarshal([]byte(doc), &cf); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		b := cf.Services["web"].Build
		if b == nil || b.Context != "./app" {
			t.Errorf("build def: got %+v", b)
		}
	}
	var cf2 ComposeFile
	doc := "services:\n  web:\n    build:\n      context: ./app\n      dockerfile: Alt\n"
	if err := yaml.Unmarshal([]byte(doc), &cf2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cf2.Services["web"].Build.Dockerfile != "Alt" {
		t.Errorf("build map form: %+v", cf2.Services["web"].Build)
	}
}
