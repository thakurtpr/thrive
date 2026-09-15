//go:build !linux

package commands_test

import (
	"testing"

	"github.com/thakurprasadrout/thrive/cmd/thrive/commands"
)

// TestDistributionCmds_Use verifies all Phase B commands exist.
func TestDistributionCmds_Use(t *testing.T) {
	uses := map[string]string{
		"Save":     commands.SaveCmd().Use,
		"Load":     commands.LoadCmd().Use,
		"Import":   commands.ImportCmd().Use,
		"History":  commands.HistoryCmd().Use,
		"Login":    commands.LoginCmd().Use,
		"Logout":   commands.LogoutCmd().Use,
		"Search":   commands.SearchCmd().Use,
		"Manifest": commands.ManifestCmd().Use,
	}
	for name, use := range uses {
		if use == "" {
			t.Errorf("%sCmd: Use field is empty", name)
		}
	}
}

// TestManifestCmd_SubCommands verifies manifest sub-commands.
func TestManifestCmd_SubCommands(t *testing.T) {
	cmd := commands.ManifestCmd()
	subMap := map[string]bool{}
	for _, sub := range cmd.Commands() {
		subMap[sub.Use] = true
	}
	for _, want := range []string{"inspect [image]", "create [list] [image...]", "annotate [list] [image]", "push [list]", "rm [list]"} {
		if !subMap[want] {
			t.Errorf("ManifestCmd: missing sub-command %q (have: %v)", want, subMap)
		}
	}
}

// TestSaveCmd_OutputFlag verifies save exposes --output / -o.
func TestSaveCmd_OutputFlag(t *testing.T) {
	if f := commands.SaveCmd().Flags().Lookup("output"); f == nil {
		t.Error("SaveCmd: missing --output flag")
	}
}

// TestLoadCmd_InputFlag verifies load exposes --input / -i.
func TestLoadCmd_InputFlag(t *testing.T) {
	if f := commands.LoadCmd().Flags().Lookup("input"); f == nil {
		t.Error("LoadCmd: missing --input flag")
	}
}

// TestLoginCmd_Flags verifies login exposes --username / -u.
func TestLoginCmd_Flags(t *testing.T) {
	cmd := commands.LoginCmd()
	for _, flagName := range []string{"username", "password"} {
		if f := cmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("LoginCmd: missing --%s flag", flagName)
		}
	}
}
