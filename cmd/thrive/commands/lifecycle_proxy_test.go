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
