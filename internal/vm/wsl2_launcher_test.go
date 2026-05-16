//go:build !linux

package vm

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

type capturedCall struct {
	name string
	args []string
}

func TestWSL2Launcher_Start_ReturnsClearErrorWhenWSLMissing(t *testing.T) {
	l := &wsl2Launcher{
		runner:   func(context.Context, string, ...string) ([]byte, error) { t.Fatal("runner must not be called"); return nil, nil },
		lookPath: func(string) (string, error) { return "", exec.ErrNotFound },
	}

	_, err := l.Start(context.Background(), &Config{MemoryMB: 2048, CPUCount: 2, VMType: "wsl2"})
	if err == nil {
		t.Fatal("expected error when wsl.exe missing")
	}
	if !strings.Contains(err.Error(), "wsl") {
		t.Errorf("error should mention wsl, got: %v", err)
	}
}

func TestWSL2Launcher_Start_RegistersAndLaunchesDistro(t *testing.T) {
	var calls []capturedCall
	l := &wsl2Launcher{
		runner: func(_ context.Context, name string, args ...string) ([]byte, error) {
			calls = append(calls, capturedCall{name, args})
			return []byte("ok"), nil
		},
		lookPath:            func(string) (string, error) { return `C:\Windows\System32\wsl.exe`, nil },
		distroAlreadyExists: func(context.Context, string) bool { return false },
		rootfsPath:          func() string { return `C:\Users\test\AppData\Local\Thrive\vm\rootfs.tar.gz` },
		instanceDir:         func() string { return `C:\Users\test\AppData\Local\Thrive\vm\wsl` },
	}

	state, err := l.Start(context.Background(), &Config{MemoryMB: 2048, CPUCount: 2, VMType: "wsl2"})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if len(calls) < 2 {
		t.Fatalf("expected at least 2 wsl calls (import + run), got %d: %+v", len(calls), calls)
	}

	importArgs := strings.Join(calls[0].args, " ")
	if !strings.Contains(importArgs, "--import") {
		t.Errorf("first call should be --import, got: %s", importArgs)
	}
	if !strings.Contains(importArgs, "Thrive") {
		t.Errorf("import should use 'Thrive' distro name, got: %s", importArgs)
	}
	if !strings.Contains(importArgs, "rootfs.tar.gz") {
		t.Errorf("import should reference rootfs, got: %s", importArgs)
	}

	if state.WSLInstance != "Thrive" {
		t.Errorf("expected WSLInstance=Thrive, got %q", state.WSLInstance)
	}
	if !state.Running {
		t.Error("expected Running=true")
	}
	if state.VMType != "wsl2" {
		t.Errorf("expected VMType=wsl2, got %q", state.VMType)
	}
}

func TestWSL2Launcher_Start_SkipsImportIfDistroExists(t *testing.T) {
	var calls []capturedCall
	l := &wsl2Launcher{
		runner: func(_ context.Context, name string, args ...string) ([]byte, error) {
			calls = append(calls, capturedCall{name, args})
			return []byte("ok"), nil
		},
		lookPath:            func(string) (string, error) { return "wsl.exe", nil },
		distroAlreadyExists: func(context.Context, string) bool { return true },
		rootfsPath:          func() string { return "rootfs.tar.gz" },
		instanceDir:         func() string { return "wsl" },
	}

	_, err := l.Start(context.Background(), &Config{MemoryMB: 2048, CPUCount: 2, VMType: "wsl2"})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for _, c := range calls {
		if strings.Contains(strings.Join(c.args, " "), "--import") {
			t.Errorf("--import should be skipped when distro exists, got call: %+v", c)
		}
	}
}

func TestWSL2Launcher_Start_PropagatesImportError(t *testing.T) {
	l := &wsl2Launcher{
		runner: func(context.Context, string, ...string) ([]byte, error) {
			return []byte("WslRegisterDistribution failed"), errors.New("exit status 1")
		},
		lookPath:            func(string) (string, error) { return "wsl.exe", nil },
		distroAlreadyExists: func(context.Context, string) bool { return false },
		rootfsPath:          func() string { return "rootfs.tar.gz" },
		instanceDir:         func() string { return "wsl" },
	}

	_, err := l.Start(context.Background(), &Config{MemoryMB: 2048, CPUCount: 2, VMType: "wsl2"})
	if err == nil {
		t.Fatal("expected error on import failure")
	}
	if !strings.Contains(err.Error(), "import") {
		t.Errorf("error should mention import, got: %v", err)
	}
}

func TestWSL2Launcher_Stop_TerminatesDistro(t *testing.T) {
	var calls []capturedCall
	l := &wsl2Launcher{
		runner: func(_ context.Context, name string, args ...string) ([]byte, error) {
			calls = append(calls, capturedCall{name, args})
			return nil, nil
		},
		lookPath: func(string) (string, error) { return "wsl.exe", nil },
	}

	err := l.Stop(context.Background(), &VMState{Running: true, VMType: "wsl2", WSLInstance: "Thrive"})
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	args := strings.Join(calls[0].args, " ")
	if !strings.Contains(args, "--terminate") {
		t.Errorf("expected --terminate, got: %s", args)
	}
	if !strings.Contains(args, "Thrive") {
		t.Errorf("expected distro name, got: %s", args)
	}
}

func TestWSL2Launcher_Stop_NoOpWhenAlreadyStopped(t *testing.T) {
	called := false
	l := &wsl2Launcher{
		runner: func(context.Context, string, ...string) ([]byte, error) {
			called = true
			return nil, nil
		},
		lookPath: func(string) (string, error) { return "wsl.exe", nil },
	}

	if err := l.Stop(context.Background(), &VMState{Running: false, WSLInstance: "Thrive"}); err != nil {
		t.Fatalf("Stop on stopped VM should succeed: %v", err)
	}
	if called {
		t.Error("runner should not be called when VM not running")
	}
}
