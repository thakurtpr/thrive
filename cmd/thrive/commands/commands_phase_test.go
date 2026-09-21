//go:build linux

package commands

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestPhaseCommands_Use verifies every Phase A-E Linux command exists.
func TestPhaseCommands_Use(t *testing.T) {
	uses := map[string]string{
		"Create": CreateCmd().Use, "Pause": PauseCmd().Use,
		"Unpause": UnpauseCmd().Use, "Wait": WaitCmd().Use,
		"Rename": RenameCmd().Use, "Stats": StatsCmd().Use,
		"Update": UpdateCmd().Use, "Top": TopCmd().Use,
		"Port": PortCmd().Use, "Diff": DiffCmd().Use,
		"Export": ExportCmd().Use, "Commit": CommitCmd().Use,
		"Save": SaveCmd().Use, "Load": LoadCmd().Use,
		"Import": ImportCmd().Use, "History": HistoryCmd().Use,
		"Login": LoginCmd().Use, "Logout": LogoutCmd().Use,
		"Search": SearchCmd().Use, "Manifest": ManifestCmd().Use,
		"Network": NetworkCmd().Use, "Volume": VolumeCmd().Use,
		"Swarm": SwarmCmd().Use, "Service": ServiceCmd().Use,
		"Stack": StackCmd().Use, "Buildx": BuildxCmd().Use,
		"Context": ContextCmd().Use, "Plugin": PluginCmd().Use,
		"Checkpoint": CheckpointCmd().Use,
	}
	for name, use := range uses {
		if use == "" {
			t.Errorf("%sCmd: Use field is empty", name)
		}
	}
}

// TestSystemCmd_PhaseDSubs verifies df/events/prune sub-commands.
func TestSystemCmd_PhaseDSubs(t *testing.T) {
	subMap := map[string]bool{}
	for _, sub := range SystemCmd().Commands() {
		subMap[sub.Use] = true
	}
	for _, want := range []string{"info", "clean", "df", "events", "prune"} {
		if !subMap[want] {
			t.Errorf("SystemCmd: missing %q", want)
		}
	}
}

// TestComposeCmd_PhaseESubs verifies extended compose sub-commands.
func TestComposeCmd_PhaseESubs(t *testing.T) {
	subMap := map[string]bool{}
	for _, sub := range ComposeCmd().Commands() {
		subMap[sub.Use] = true
	}
	for _, want := range []string{
		"up", "down", "ps", "logs [service...]", "build [service...]",
		"pull [service...]", "stop [service...]", "start [service...]",
		"kill [service...]", "restart [service...]", "rm [service...]",
		"exec [service] [command...]", "config",
	} {
		if !subMap[want] {
			t.Errorf("ComposeCmd: missing %q", want)
		}
	}
}

// TestServiceCmd_Subs verifies service sub-commands.
func TestServiceCmd_Subs(t *testing.T) {
	subMap := map[string]bool{}
	for _, sub := range ServiceCmd().Commands() {
		subMap[sub.Use] = true
	}
	for _, want := range []string{
		"create [image] [command...]", "ls", "inspect [service]", "ps [service]",
		"rm [service]", "scale [service=num...]", "logs [service]",
		"update [service]", "rollback [service]",
	} {
		if !subMap[want] {
			t.Errorf("ServiceCmd: missing %q", want)
		}
	}
}

// TestParseMemory_Units verifies memory parsing for update.
func TestParseMemory_Units(t *testing.T) {
	cases := map[string]int64{
		"512": 512, "1k": 1024, "512m": 512 << 20, "1g": 1 << 30, "2G": 2 << 30,
	}
	for in, want := range cases {
		got, err := parseMemory(in)
		if err != nil || got != want {
			t.Errorf("parseMemory(%q): got %d, %v; want %d", in, got, err, want)
		}
	}
	if _, err := parseMemory("bogus"); err == nil {
		t.Error("parseMemory(bogus): expected error, got nil")
	}
}

// TestSplitScale verifies scale parsing for up/service.
func TestSplitScale(t *testing.T) {
	name, n, ok := splitScale("web=3")
	if !ok || name != "web" || n != 3 {
		t.Errorf("splitScale: got %q %d %v", name, n, ok)
	}
	if _, _, ok := splitScale("web"); ok {
		t.Error("splitScale(web): expected false, got true")
	}
	if _, _, ok := splitScale("web=-1"); ok {
		t.Error("splitScale negative: expected false, got true")
	}
}

// TestCreateCmd_Flags verifies Linux create exposes the full flag set.
// Must match TestCreateCmd_ParityFlags (proxy) — change both together.
func TestCreateCmd_Flags(t *testing.T) {
	cmd := CreateCmd()
	for _, flagName := range []string{
		"name", "env", "secret", "config", "publish", "volume", "network",
		"memory", "cpus", "cpu-shares", "pids-limit", "restart",
	} {
		if f := cmd.Flags().Lookup(flagName); f == nil {
			t.Errorf("CreateCmd: missing --%s flag", flagName)
		}
	}
}

// TestServiceCreateCmd_Flags verifies Linux service create exposes the full
// flag set. Must match TestServiceCreateCmd_ParityFlags (proxy) — change
// both together.
func TestServiceCreateCmd_Flags(t *testing.T) {
	var create *cobra.Command
	for _, sub := range ServiceCmd().Commands() {
		if sub.Use == "create [image] [command...]" {
			create = sub
		}
	}
	if create == nil {
		t.Fatal("ServiceCmd: missing create subcommand")
	}
	for _, flagName := range []string{
		"name", "replicas", "env", "secret", "config", "publish", "volume",
		"network", "restart", "update-parallelism", "update-delay",
	} {
		if f := create.Flags().Lookup(flagName); f == nil {
			t.Errorf("service create: missing --%s flag", flagName)
		}
	}
}

// TestR2FlagDepth verifies R2 Docker-parity flags exist.
func TestR2FlagDepth(t *testing.T) {
	for _, flagName := range []string{"platform", "quiet", "all-tags", "verify", "verify-key"} {
		if f := PullCmd().Flags().Lookup(flagName); f == nil {
			t.Errorf("PullCmd: missing --%s flag", flagName)
		}
	}
	if f := PushCmd().Flags().Lookup("quiet"); f == nil {
		t.Error("PushCmd: missing --quiet flag")
	}
	for _, flagName := range []string{"author", "message", "pause"} {
		if f := CommitCmd().Flags().Lookup(flagName); f == nil {
			t.Errorf("CommitCmd: missing --%s flag", flagName)
		}
	}
	for _, flagName := range []string{"since", "until", "timestamps"} {
		if f := LogsCmd().Flags().Lookup(flagName); f == nil {
			t.Errorf("LogsCmd: missing --%s flag", flagName)
		}
	}
	if f := StatsCmd().Flags().Lookup("no-stream"); f == nil {
		t.Error("StatsCmd: missing --no-stream flag")
	}
}
