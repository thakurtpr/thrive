//go:build !linux

package vm

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestHyperVLauncher_Start_ReturnsErrorWhenPowerShellMissing(t *testing.T) {
	l := &hyperVLauncher{
		runner:   func(context.Context, string, ...string) ([]byte, error) { t.Fatal("runner must not be called"); return nil, nil },
		lookPath: func(string) (string, error) { return "", exec.ErrNotFound },
	}

	_, err := l.Start(context.Background(), &Config{MemoryMB: 2048, CPUCount: 2, VMType: "hyperv"})
	if err == nil {
		t.Fatal("expected error when powershell missing")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "powershell") {
		t.Errorf("error should mention powershell, got: %v", err)
	}
}

func TestHyperVLauncher_Start_CreatesAndStartsVM(t *testing.T) {
	var calls []capturedCall
	l := &hyperVLauncher{
		runner: func(_ context.Context, name string, args ...string) ([]byte, error) {
			calls = append(calls, capturedCall{name, args})
			return []byte("ok"), nil
		},
		lookPath:        func(string) (string, error) { return `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`, nil },
		vmAlreadyExists: func(context.Context, string) bool { return false },
		vhdPath:         func() string { return `C:\Users\test\AppData\Local\Thrive\vm\disk.vhdx` },
	}

	state, err := l.Start(context.Background(), &Config{MemoryMB: 4096, CPUCount: 4, VMType: "hyperv"})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if len(calls) < 2 {
		t.Fatalf("expected New-VM + Start-VM (at least 2 calls), got %d", len(calls))
	}

	sawNewVM := false
	sawStartVM := false
	for _, c := range calls {
		joined := strings.Join(c.args, " ")
		if strings.Contains(joined, "New-VM") {
			sawNewVM = true
			if !strings.Contains(joined, "Thrive") {
				t.Errorf("New-VM should use 'Thrive' name, got: %s", joined)
			}
			if !strings.Contains(joined, "4096") {
				t.Errorf("New-VM should pass memory, got: %s", joined)
			}
		}
		if strings.Contains(joined, "Start-VM") {
			sawStartVM = true
		}
	}
	if !sawNewVM {
		t.Error("expected a New-VM invocation")
	}
	if !sawStartVM {
		t.Error("expected a Start-VM invocation")
	}

	if !state.Running {
		t.Error("expected Running=true")
	}
	if state.VMType != "hyperv" {
		t.Errorf("expected VMType=hyperv, got %q", state.VMType)
	}
}

func TestHyperVLauncher_Start_SkipsCreateIfVMExists(t *testing.T) {
	var calls []capturedCall
	l := &hyperVLauncher{
		runner: func(_ context.Context, name string, args ...string) ([]byte, error) {
			calls = append(calls, capturedCall{name, args})
			return nil, nil
		},
		lookPath:        func(string) (string, error) { return "powershell.exe", nil },
		vmAlreadyExists: func(context.Context, string) bool { return true },
		vhdPath:         func() string { return "disk.vhdx" },
	}

	_, err := l.Start(context.Background(), &Config{MemoryMB: 2048, CPUCount: 2, VMType: "hyperv"})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for _, c := range calls {
		if strings.Contains(strings.Join(c.args, " "), "New-VM") {
			t.Errorf("New-VM should not be called when VM exists: %+v", c)
		}
	}
}

func TestHyperVLauncher_Start_PropagatesCreateError(t *testing.T) {
	l := &hyperVLauncher{
		runner: func(context.Context, string, ...string) ([]byte, error) {
			return []byte("Hyper-V is not enabled"), errors.New("exit status 1")
		},
		lookPath:        func(string) (string, error) { return "powershell.exe", nil },
		vmAlreadyExists: func(context.Context, string) bool { return false },
		vhdPath:         func() string { return "disk.vhdx" },
	}

	_, err := l.Start(context.Background(), &Config{MemoryMB: 2048, CPUCount: 2, VMType: "hyperv"})
	if err == nil {
		t.Fatal("expected error when New-VM fails")
	}
}

func TestHyperVLauncher_Stop_CallsStopVM(t *testing.T) {
	var calls []capturedCall
	l := &hyperVLauncher{
		runner: func(_ context.Context, name string, args ...string) ([]byte, error) {
			calls = append(calls, capturedCall{name, args})
			return nil, nil
		},
		lookPath: func(string) (string, error) { return "powershell.exe", nil },
	}

	err := l.Stop(context.Background(), &VMState{Running: true, VMType: "hyperv"})
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if !strings.Contains(strings.Join(calls[0].args, " "), "Stop-VM") {
		t.Errorf("expected Stop-VM, got: %s", strings.Join(calls[0].args, " "))
	}
}

func TestHyperVLauncher_Stop_NoOpWhenStopped(t *testing.T) {
	called := false
	l := &hyperVLauncher{
		runner: func(context.Context, string, ...string) ([]byte, error) {
			called = true
			return nil, nil
		},
		lookPath: func(string) (string, error) { return "powershell.exe", nil },
	}
	if err := l.Stop(context.Background(), &VMState{Running: false}); err != nil {
		t.Fatalf("Stop on stopped VM should succeed: %v", err)
	}
	if called {
		t.Error("runner should not be called when VM not running")
	}
}
