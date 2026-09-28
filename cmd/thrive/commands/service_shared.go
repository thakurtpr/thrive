package commands

import (
	"github.com/spf13/cobra"
)

// serviceCreateOpts reads the service-create flag set into the daemon
// bridge map. Portable and pure (no dial), so tests assert the mapping.
// Kept as a helper (not inlined in RunE) for the same reason as
// containerOpts: the wire format gets unit coverage on every platform.
func serviceCreateOpts(cmd *cobra.Command) map[string]any {
	name, _ := cmd.Flags().GetString("name")
	replicas, _ := cmd.Flags().GetInt("replicas")
	envVars, _ := cmd.Flags().GetStringArray("env")
	secrets, _ := cmd.Flags().GetStringArray("secret")
	configs, _ := cmd.Flags().GetStringArray("config")
	ports, _ := cmd.Flags().GetStringArray("publish")
	volumes, _ := cmd.Flags().GetStringArray("volume")
	netMode, _ := cmd.Flags().GetString("network")
	parallelism, _ := cmd.Flags().GetInt("update-parallelism")
	delay, _ := cmd.Flags().GetInt("update-delay")
	restart, _ := cmd.Flags().GetString("restart")
	return map[string]any{
		"name": name, "replicas": replicas, "env": envVars, "ports": ports,
		"volumes": volumes, "network": netMode, "secrets": secrets,
		"configs":     configs,
		"parallelism": parallelism, "delay": delay, "restart": restart,
	}
}
