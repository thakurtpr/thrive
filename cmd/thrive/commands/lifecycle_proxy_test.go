//go:build !linux

package commands_test

import (
	"testing"

	"github.com/thakurprasadrout/thrive/cmd/thrive/commands"
)

// TestLifecycleProxyCmds_Use verifies all Phase A proxy commands exist with
// non-empty Use fields.
func TestLifecycleProxyCmds_Use(t *testing.T) {
	uses := map[string]string{
		"Create":  commands.CreateCmd().Use,
		"Pause":   commands.PauseCmd().Use,
		"Unpause": commands.UnpauseCmd().Use,
		"Wait":    commands.WaitCmd().Use,
		"Rename":  commands.RenameCmd().Use,
		"Stats":   commands.StatsCmd().Use,
		"Update":  commands.UpdateCmd().Use,
		"Top":     commands.TopCmd().Use,
		"Port":    commands.PortCmd().Use,
		"Diff":    commands.DiffCmd().Use,
		"Export":  commands.ExportCmd().Use,
		"Commit":  commands.CommitCmd().Use,
	}
	for name, use := range uses {
		if use == "" {
			t.Errorf("%sCmd: Use field is empty", name)
		}
	}
}

// TestUpdateCmd_Flags verifies UpdateCmd exposes resource-limit flags.
func TestUpdateCmd_Flags(t *testing.T) {
	cmd := commands.UpdateCmd()
	for _, flagName := range []string{"memory", "cpu-quota", "cpu-shares", "pids-limit"} {
		if f := cmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("UpdateCmd: missing --%s flag", flagName)
		}
	}
}

// TestExportCmd_OutputFlag verifies ExportCmd exposes --output / -o.
func TestExportCmd_OutputFlag(t *testing.T) {
	cmd := commands.ExportCmd()
	if f := cmd.Flags().Lookup("output"); f == nil {
		t.Error("ExportCmd: missing --output flag")
	}
}

// TestCommitCmd_R2Flags verifies the commit proxy exposes -a/-m/-p parity flags.
func TestCommitCmd_R2Flags(t *testing.T) {
	cmd := commands.CommitCmd()
	for _, flagName := range []string{"author", "message", "pause"} {
		if f := cmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("CommitCmd proxy: missing --%s flag", flagName)
		}
	}
}

// TestPullCmd_R3Flags verifies the pull proxy exposes verify + stats
// streaming parity flags on non-Linux platforms.
func TestPullCmd_R3Flags(t *testing.T) {
	cmd := commands.PullCmd()
	for _, flagName := range []string{"platform", "quiet", "all-tags", "verify", "verify-key"} {
		if f := cmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("PullCmd proxy: missing --%s flag", flagName)
		}
	}
	if f := commands.StatsCmd().Flags().Lookup("no-stream"); f == nil {
		t.Error("StatsCmd proxy: missing --no-stream flag")
	}
}

// TestLogsCmd_R2Flags verifies the logs proxy exposes time-filter flags
// (registered for parity, honestly refused at runtime).
func TestLogsCmd_R2Flags(t *testing.T) {
	cmd := commands.LogsCmd()
	for _, flagName := range []string{"since", "until", "timestamps"} {
		if f := cmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("LogsCmd proxy: missing --%s flag", flagName)
		}
	}
}
