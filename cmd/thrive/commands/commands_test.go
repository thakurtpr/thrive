//go:build !linux

// Package commands_test provides CLI integration tests for the commands package.
// All tests use the !linux build tag so they run on macOS and Windows (where
// proxy/stub variants are compiled in) without requiring a running Linux VM.
//
// Tests verify command structure (Use, flags, sub-commands) rather than
// business logic, keeping the suite fast and hermetic.
package commands_test

import (
	"testing"

	"github.com/thakurprasadrout/thrive/cmd/thrive/commands"
)

// ---------------------------------------------------------------------------
// PsCmd
// ---------------------------------------------------------------------------

// TestPsCmd_Use verifies that PsCmd returns a command with a non-empty Use.
func TestPsCmd_Use(t *testing.T) {
	cmd := commands.PsCmd()
	if cmd == nil {
		t.Fatal("PsCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("PsCmd: Use field is empty")
	}
}

// TestPsCmd_HasShortDescription verifies PsCmd has a human-readable summary.
func TestPsCmd_HasShortDescription(t *testing.T) {
	cmd := commands.PsCmd()
	if cmd.Short == "" {
		t.Error("PsCmd: Short description is empty")
	}
}

// ---------------------------------------------------------------------------
// LogsCmd
// ---------------------------------------------------------------------------

// TestLogsCmd_Use verifies that LogsCmd returns a command with a non-empty Use.
func TestLogsCmd_Use(t *testing.T) {
	cmd := commands.LogsCmd()
	if cmd == nil {
		t.Fatal("LogsCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("LogsCmd: Use field is empty")
	}
}

// TestLogsCmd_FollowFlag verifies that LogsCmd exposes the --follow / -f flag
// with a default value of false.
func TestLogsCmd_FollowFlag(t *testing.T) {
	cmd := commands.LogsCmd()
	f := cmd.Flags().Lookup("follow")
	if f == nil {
		t.Fatal("LogsCmd: missing --follow flag")
	}
	if f.DefValue != "false" {
		t.Errorf("LogsCmd --follow default: got %q, want %q", f.DefValue, "false")
	}
}

// ---------------------------------------------------------------------------
// RunCmd
// ---------------------------------------------------------------------------

// TestRunCmd_Use verifies that RunCmd has a non-empty Use field.
func TestRunCmd_Use(t *testing.T) {
	cmd := commands.RunCmd()
	if cmd == nil {
		t.Fatal("RunCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("RunCmd: Use field is empty")
	}
}

// TestRunCmd_DetachFlag verifies that RunCmd exposes --detach / -d.
func TestRunCmd_DetachFlag(t *testing.T) {
	cmd := commands.RunCmd()
	f := cmd.Flags().Lookup("detach")
	if f == nil {
		t.Fatal("RunCmd: missing --detach flag")
	}
	if f.DefValue != "false" {
		t.Errorf("RunCmd --detach default: got %q, want %q", f.DefValue, "false")
	}
}

// TestRunCmd_RmFlag verifies that RunCmd exposes the --rm flag.
func TestRunCmd_RmFlag(t *testing.T) {
	cmd := commands.RunCmd()
	if f := cmd.Flags().Lookup("rm"); f == nil {
		t.Fatal("RunCmd: missing --rm flag")
	}
}

// TestRunCmd_EnvFlag verifies that RunCmd exposes --env / -e.
func TestRunCmd_EnvFlag(t *testing.T) {
	cmd := commands.RunCmd()
	if f := cmd.Flags().Lookup("env"); f == nil {
		t.Fatal("RunCmd: missing --env flag")
	}
}

// TestRunCmd_NameFlag verifies that RunCmd exposes --name.
func TestRunCmd_NameFlag(t *testing.T) {
	cmd := commands.RunCmd()
	if f := cmd.Flags().Lookup("name"); f == nil {
		t.Fatal("RunCmd: missing --name flag")
	}
}

// ---------------------------------------------------------------------------
// KillCmd
// ---------------------------------------------------------------------------

// TestKillCmd_Use verifies KillCmd has a non-empty Use field.
func TestKillCmd_Use(t *testing.T) {
	cmd := commands.KillCmd()
	if cmd == nil {
		t.Fatal("KillCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("KillCmd: Use field is empty")
	}
}

// TestKillCmd_HasArgValidator verifies KillCmd enforces a non-nil Args validator.
func TestKillCmd_HasArgValidator(t *testing.T) {
	cmd := commands.KillCmd()
	if cmd.Args == nil {
		t.Error("KillCmd: Args validator is nil — expected ExactArgs(1)")
	}
}

// ---------------------------------------------------------------------------
// RmCmd
// ---------------------------------------------------------------------------

// TestRmCmd_Use verifies RmCmd has a non-empty Use field.
func TestRmCmd_Use(t *testing.T) {
	cmd := commands.RmCmd()
	if cmd == nil {
		t.Fatal("RmCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("RmCmd: Use field is empty")
	}
}

// ---------------------------------------------------------------------------
// SystemCmd
// ---------------------------------------------------------------------------

// TestSystemCmd_Use verifies SystemCmd has a non-empty Use field.
func TestSystemCmd_Use(t *testing.T) {
	cmd := commands.SystemCmd()
	if cmd == nil {
		t.Fatal("SystemCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("SystemCmd: Use field is empty")
	}
}

// ---------------------------------------------------------------------------
// DesktopCmd
// ---------------------------------------------------------------------------

// TestDesktopCmd_Use verifies DesktopCmd has a non-empty Use field.
func TestDesktopCmd_Use(t *testing.T) {
	cmd := commands.DesktopCmd()
	if cmd == nil {
		t.Fatal("DesktopCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("DesktopCmd: Use field is empty")
	}
}

// TestDesktopCmd_SubCommands verifies DesktopCmd registers the lifecycle
// sub-commands: init, start, stop, restart, status.
func TestDesktopCmd_SubCommands(t *testing.T) {
	cmd := commands.DesktopCmd()
	subs := cmd.Commands()
	if len(subs) == 0 {
		t.Fatal("DesktopCmd: has no sub-commands")
	}

	want := map[string]bool{
		"init":    false,
		"start":   false,
		"stop":    false,
		"restart": false,
		"status":  false,
	}
	for _, sub := range subs {
		want[sub.Use] = true
	}
	for name, found := range want {
		if !found {
			t.Errorf("DesktopCmd: missing sub-command %q", name)
		}
	}
}

// ---------------------------------------------------------------------------
// BuildCmd / PushCmd / PullCmd (stub variants on !linux)
// ---------------------------------------------------------------------------

// TestBuildCmd_Use verifies BuildCmd has a non-empty Use field on non-linux.
func TestBuildCmd_Use(t *testing.T) {
	cmd := commands.BuildCmd()
	if cmd == nil {
		t.Fatal("BuildCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("BuildCmd: Use field is empty")
	}
}

// TestPushCmd_Use verifies PushCmd has a non-empty Use field on non-linux.
func TestPushCmd_Use(t *testing.T) {
	cmd := commands.PushCmd()
	if cmd == nil {
		t.Fatal("PushCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("PushCmd: Use field is empty")
	}
}

// TestPullCmd_Use verifies PullCmd has a non-empty Use field on non-linux.
func TestPullCmd_Use(t *testing.T) {
	cmd := commands.PullCmd()
	if cmd == nil {
		t.Fatal("PullCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("PullCmd: Use field is empty")
	}
}

// TestPushCmd_RegistryFlags verifies PushCmd exposes --username and --password.
func TestPushCmd_RegistryFlags(t *testing.T) {
	cmd := commands.PushCmd()
	for _, flagName := range []string{"username", "password"} {
		if f := cmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("PushCmd: missing --%s flag", flagName)
		}
	}
}

// TestPullCmd_RegistryFlags verifies PullCmd exposes --username and --password.
func TestPullCmd_RegistryFlags(t *testing.T) {
	cmd := commands.PullCmd()
	for _, flagName := range []string{"username", "password"} {
		if f := cmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("PullCmd: missing --%s flag", flagName)
		}
	}
}

// ---------------------------------------------------------------------------
// ImagesCmd / RmiCmd (stub variants on !linux)
// ---------------------------------------------------------------------------

// TestImagesCmd_Use verifies ImagesCmd has a non-empty Use field on non-linux.
func TestImagesCmd_Use(t *testing.T) {
	cmd := commands.ImagesCmd()
	if cmd == nil {
		t.Fatal("ImagesCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("ImagesCmd: Use field is empty")
	}
}

// TestRmiCmd_Use verifies RmiCmd has a non-empty Use field on non-linux.
func TestRmiCmd_Use(t *testing.T) {
	cmd := commands.RmiCmd()
	if cmd == nil {
		t.Fatal("RmiCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("RmiCmd: Use field is empty")
	}
}

// ---------------------------------------------------------------------------
// MetricsCmd / SecretCmd (stub variants on !linux)
// ---------------------------------------------------------------------------

// TestMetricsCmd_Use verifies MetricsCmd has a non-empty Use field on non-linux.
func TestMetricsCmd_Use(t *testing.T) {
	cmd := commands.MetricsCmd()
	if cmd == nil {
		t.Fatal("MetricsCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("MetricsCmd: Use field is empty")
	}
}

// TestSecretCmd_SubCommands verifies SecretCmd registers create, ls, and rm
// sub-commands on non-linux (stub variant).
func TestSecretCmd_SubCommands(t *testing.T) {
	cmd := commands.SecretCmd()
	if cmd == nil {
		t.Fatal("SecretCmd: returned nil")
	}
	subs := cmd.Commands()
	if len(subs) == 0 {
		t.Fatal("SecretCmd: has no sub-commands")
	}

	want := map[string]bool{
		"create [name] [value]": false,
		"ls":                    false,
		"rm [name]":             false,
	}
	for _, sub := range subs {
		want[sub.Use] = true
	}
	for name, found := range want {
		if !found {
			t.Errorf("SecretCmd: missing sub-command with Use=%q", name)
		}
	}
}

// ---------------------------------------------------------------------------
// StartCmd / StopCmd / RestartCmd
// ---------------------------------------------------------------------------

func TestStartCmd_Use(t *testing.T) {
	cmd := commands.StartCmd()
	if cmd == nil {
		t.Fatal("StartCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("StartCmd: Use field is empty")
	}
}

func TestStopCmd_Use(t *testing.T) {
	cmd := commands.StopCmd()
	if cmd == nil {
		t.Fatal("StopCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("StopCmd: Use field is empty")
	}
}

func TestRestartCmd_Use(t *testing.T) {
	cmd := commands.RestartCmd()
	if cmd == nil {
		t.Fatal("RestartCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("RestartCmd: Use field is empty")
	}
}

// ---------------------------------------------------------------------------
// ExecCmd / CpCmd / InspectCmd
// ---------------------------------------------------------------------------

func TestExecCmd_Use(t *testing.T) {
	cmd := commands.ExecCmd()
	if cmd == nil {
		t.Fatal("ExecCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("ExecCmd: Use field is empty")
	}
}

func TestCpCmd_Use(t *testing.T) {
	cmd := commands.CpCmd()
	if cmd == nil {
		t.Fatal("CpCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("CpCmd: Use field is empty")
	}
}

func TestInspectCmd_Use(t *testing.T) {
	cmd := commands.InspectCmd()
	if cmd == nil {
		t.Fatal("InspectCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("InspectCmd: Use field is empty")
	}
}

// ---------------------------------------------------------------------------
// TagCmd / ComposeCmd / SignCmd
// ---------------------------------------------------------------------------

func TestTagCmd_Use(t *testing.T) {
	cmd := commands.TagCmd()
	if cmd == nil {
		t.Fatal("TagCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("TagCmd: Use field is empty")
	}
}

func TestComposeCmd_Use(t *testing.T) {
	cmd := commands.ComposeCmd()
	if cmd == nil {
		t.Fatal("ComposeCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("ComposeCmd: Use field is empty")
	}
}

func TestSignCmd_Use(t *testing.T) {
	cmd := commands.SignCmd()
	if cmd == nil {
		t.Fatal("SignCmd: returned nil")
	}
	if cmd.Use == "" {
		t.Error("SignCmd: Use field is empty")
	}
}

func TestSignCmd_SubCommands(t *testing.T) {
	cmd := commands.SignCmd()
	subMap := map[string]bool{}
	for _, sub := range cmd.Commands() {
		subMap[sub.Use] = true
	}
	for _, want := range []string{"keygen", "verify IMAGE"} {
		if !subMap[want] {
			t.Errorf("SignCmd: missing sub-command %q (have: %v)", want, subMap)
		}
	}
}
