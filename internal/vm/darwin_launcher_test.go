//go:build !linux

package vm

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDarwinLauncher_Start_ReturnsClearErrorWhenVfkitMissing(t *testing.T) {
	l := &darwinLauncher{
		starter:  func(string, ...string) (int, error) { t.Fatal("starter must not be called when binary missing"); return 0, nil },
		lookPath: func(file string) (string, error) { return "", exec.ErrNotFound },
	}

	_, err := l.Start(context.Background(), &Config{MemoryMB: 2048, CPUCount: 2, VMType: "darwin-hv"})
	if err == nil {
		t.Fatal("expected error when vfkit missing, got nil")
	}
	if !strings.Contains(err.Error(), "vfkit") {
		t.Errorf("error should mention vfkit, got: %v", err)
	}
	if !strings.Contains(err.Error(), "thrive desktop init") {
		t.Errorf("error should suggest thrive desktop init, got: %v", err)
	}
}

func TestDarwinLauncher_Start_SpawnsVfkitWithCorrectArgs(t *testing.T) {
	var (
		capturedName string
		capturedArgs []string
	)
	l := &darwinLauncher{
		starter: func(name string, args ...string) (int, error) {
			capturedName = name
			capturedArgs = args
			return 4242, nil
		},
		lookPath: func(file string) (string, error) { return "/opt/homebrew/bin/vfkit", nil },
	}

	state, err := l.Start(context.Background(), &Config{MemoryMB: 4096, CPUCount: 4, VMType: "darwin-hv"})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if capturedName != "/opt/homebrew/bin/vfkit" {
		t.Errorf("expected vfkit binary path, got %q", capturedName)
	}

	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "--memory 4096") {
		t.Errorf("expected memory arg, got: %s", joined)
	}
	if !strings.Contains(joined, "--cpus 4") {
		t.Errorf("expected cpus arg, got: %s", joined)
	}
	if !strings.Contains(joined, "--bootloader") {
		t.Errorf("expected bootloader arg, got: %s", joined)
	}

	if state.PID != 4242 {
		t.Errorf("expected PID 4242, got %d", state.PID)
	}
	if !state.Running {
		t.Error("expected Running=true")
	}
	if state.VMType != "darwin-hv" {
		t.Errorf("expected VMType darwin-hv, got %q", state.VMType)
	}
}

func TestDarwinLauncher_Start_PropagatesStarterError(t *testing.T) {
	l := &darwinLauncher{
		starter:  func(string, ...string) (int, error) { return 0, errors.New("fork failed") },
		lookPath: func(file string) (string, error) { return "/usr/bin/vfkit", nil },
	}

	_, err := l.Start(context.Background(), &Config{MemoryMB: 2048, CPUCount: 2, VMType: "darwin-hv"})
	if err == nil {
		t.Fatal("expected error when starter fails")
	}
	if !strings.Contains(err.Error(), "fork failed") {
		t.Errorf("error should wrap starter error, got: %v", err)
	}
}

func TestDarwinLauncher_Stop_KillsRunningProcess(t *testing.T) {
	var killed int
	l := &darwinLauncher{
		killProcess: func(pid int) error {
			killed = pid
			return nil
		},
	}

	err := l.Stop(context.Background(), &VMState{Running: true, PID: 7777, VMType: "darwin-hv"})
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if killed != 7777 {
		t.Errorf("expected to kill PID 7777, got %d", killed)
	}
}

func TestDarwinLauncher_Stop_NoOpWhenAlreadyStopped(t *testing.T) {
	called := false
	l := &darwinLauncher{
		killProcess: func(pid int) error {
			called = true
			return nil
		},
	}

	err := l.Stop(context.Background(), &VMState{Running: false, PID: 7777})
	if err != nil {
		t.Fatalf("Stop on stopped VM should succeed: %v", err)
	}
	if called {
		t.Error("killProcess should not be called when VM is not running")
	}
}

func TestDarwinLauncher_Start_UsesBundledVfkitWhenPresent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Place a mock vfkit binary at the bundled path ThriveDir()/vm/vfkit
	bundledDir := filepath.Join(home, ".thrive", "vm")
	if err := os.MkdirAll(bundledDir, 0755); err != nil {
		t.Fatalf("failed to create vm dir: %v", err)
	}
	bundledVfkit := filepath.Join(bundledDir, "vfkit")
	if err := os.WriteFile(bundledVfkit, []byte("#!/bin/sh\necho mock"), 0755); err != nil {
		t.Fatalf("failed to write mock vfkit: %v", err)
	}

	var capturedName string
	lookPathCalled := false
	l := &darwinLauncher{
		starter: func(name string, args ...string) (int, error) {
			capturedName = name
			return 1234, nil
		},
		lookPath: func(file string) (string, error) {
			lookPathCalled = true
			return "", exec.ErrNotFound
		},
	}

	_, err := l.Start(context.Background(), &Config{MemoryMB: 2048, CPUCount: 2, VMType: "darwin-hv"})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if capturedName != bundledVfkit {
		t.Errorf("expected bundled vfkit path %q, got %q", bundledVfkit, capturedName)
	}
	if lookPathCalled {
		t.Error("lookPath should not be called when bundled vfkit exists")
	}
}

func TestDarwinLauncher_Start_FallsBackToPathWhenBundledMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// No bundled vfkit placed — fallback to PATH

	var capturedName string
	l := &darwinLauncher{
		starter: func(name string, args ...string) (int, error) {
			capturedName = name
			return 5678, nil
		},
		lookPath: func(file string) (string, error) { return "/usr/local/bin/vfkit", nil },
	}

	_, err := l.Start(context.Background(), &Config{MemoryMB: 2048, CPUCount: 2, VMType: "darwin-hv"})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if capturedName != "/usr/local/bin/vfkit" {
		t.Errorf("expected PATH vfkit, got %q", capturedName)
	}
}
