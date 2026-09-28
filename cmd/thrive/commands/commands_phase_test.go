//go:build linux

package commands

import (
	"strings"
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

// TestBuildxCmd_Flags verifies Linux buildx build/bake expose their flags.
// Must match TestBuildxRefuseCmds_Flags (proxy) — change both together.
func TestBuildxCmd_Flags(t *testing.T) {
	subs := map[string]*cobra.Command{}
	for _, sub := range BuildxCmd().Commands() {
		subs[sub.Use] = sub
	}
	build, ok := subs["build [path]"]
	if !ok {
		t.Fatal("BuildxCmd: missing build subcommand")
	}
	for _, flagName := range []string{"platform", "file", "tag", "no-cache"} {
		if f := build.Flags().Lookup(flagName); f == nil {
			t.Errorf("buildx build: missing --%s flag", flagName)
		}
	}
	bake, ok := subs["bake [service...]"]
	if !ok {
		t.Fatal("BuildxCmd: missing bake subcommand")
	}
	if f := bake.Flags().Lookup("file"); f == nil {
		t.Error("buildx bake: missing --file flag")
	}
}

// TestComposeUp_Flags verifies Linux compose up exposes --scale/--build.
// Must match TestComposeUp_BuildFlag (proxy) — change both together.
func TestComposeUp_Flags(t *testing.T) {
	var up *cobra.Command
	for _, sub := range ComposeCmd().Commands() {
		if sub.Use == "up" {
			up = sub
		}
	}
	if up == nil {
		t.Fatal("ComposeCmd: missing up subcommand")
	}
	for _, flagName := range []string{"scale", "build"} {
		if f := up.Flags().Lookup(flagName); f == nil {
			t.Errorf("compose up: missing --%s flag", flagName)
		}
	}
}

// TestComposeServiceOps_Flags verifies Linux compose service ops expose
// timeout/signal/force. Must match the proxy TestComposeServiceOps_Flags —
// change both together.
func TestComposeServiceOps_Flags(t *testing.T) {
	subs := map[string]*cobra.Command{}
	for _, sub := range ComposeCmd().Commands() {
		subs[sub.Use] = sub
	}
	cases := map[string][]string{
		"stop [service...]":    {"timeout"},
		"kill [service...]":    {"signal"},
		"restart [service...]": {"timeout"},
		"rm [service...]":      {"force"},
	}
	for use, flags := range cases {
		sub, ok := subs[use]
		if !ok {
			t.Fatalf("ComposeCmd: missing %q subcommand", use)
		}
		for _, flagName := range flags {
			if f := sub.Flags().Lookup(flagName); f == nil {
				t.Errorf("compose %q: missing --%s flag", use, flagName)
			}
		}
	}
}

// TestRmCmd_ForceFlag verifies Linux rm exposes --force (docker parity:
// refuse running containers unless forced).
func TestRmCmd_ForceFlag(t *testing.T) {
	if f := RmCmd().Flags().Lookup("force"); f == nil {
		t.Error("RmCmd: missing --force flag")
	}
}

// TestBuildNsenterArgs verifies the nsenter prefix pins --root to the
// target's root (otherwise fs writes land on the host).
func TestBuildNsenterArgs(t *testing.T) {
	args := buildNsenterArgs(485)
	joined := strings.Join(args, " ")
	// NOTE: --root uses the = form: nsenter declares it with an optional
	// argument, so space-separated --root <path> is parsed as the command.
	for _, want := range []string{"--target 485", "--mount", "--root=/proc/485/root", "--"} {
		if !strings.Contains(joined, want) {
			t.Errorf("nsenter args %q: missing %q", joined, want)
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
