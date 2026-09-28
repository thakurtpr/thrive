package commands

import (
	"strings"

	"github.com/spf13/cobra"
)

// Shared container-option builders for the !linux run/create proxies
// (run_proxy.go, lifecycle_proxy.go). Portable and pure: reads parsed flag
// state into the bridge opts map without touching the daemon, so unit
// tests can assert the mapping directly.

// parseProxyPortSpecs converts "8080:80/tcp" → map[string]any for JSON transport.
func parseProxyPortSpecs(specs []string) []map[string]any {
	var ports []map[string]any
	for _, spec := range specs {
		proto := "tcp"
		if idx := strings.LastIndex(spec, "/"); idx >= 0 {
			proto = spec[idx+1:]
			spec = spec[:idx]
		}
		idx := strings.Index(spec, ":")
		if idx < 0 {
			continue
		}
		hostPort := proxyParsePort(spec[:idx])
		ctrPort := proxyParsePort(spec[idx+1:])
		if hostPort == 0 || ctrPort == 0 {
			continue
		}
		ports = append(ports, map[string]any{
			"host_port":      hostPort,
			"container_port": ctrPort,
			"protocol":       proto,
		})
	}
	return ports
}

func proxyParsePort(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// containerOpts reads the common create/run flag set into bridge opts.
// With runFlags it also includes detach/rm/tty/interactive (run only);
// create shares everything else. Only non-zero values are sent so the
// daemon's defaults apply untouched.
func containerOpts(cmd *cobra.Command, runFlags bool) map[string]any {
	opts := map[string]any{}
	if runFlags {
		opts["detach"] = cmd.Flag("detach").Value.String() == "true"
		opts["rm"] = cmd.Flag("rm").Value.String() == "true"
		opts["tty"] = cmd.Flag("tty").Value.String() == "true"
		opts["interactive"] = cmd.Flag("interactive").Value.String() == "true"
	}
	if v := cmd.Flag("name").Value.String(); v != "" {
		opts["name"] = v
	}
	if v, _ := cmd.Flags().GetStringArray("env"); len(v) > 0 {
		opts["env"] = v
	}
	if v, _ := cmd.Flags().GetStringArray("secret"); len(v) > 0 {
		opts["secrets"] = v
	}
	if v, _ := cmd.Flags().GetStringArray("config"); len(v) > 0 {
		opts["configs"] = v
	}
	if v, _ := cmd.Flags().GetStringArray("publish"); len(v) > 0 {
		opts["ports"] = parseProxyPortSpecs(v)
	}
	if v, _ := cmd.Flags().GetStringArray("volume"); len(v) > 0 {
		opts["volumes"] = v
	}
	if v := cmd.Flag("network").Value.String(); v != "" {
		opts["network"] = v
	}
	if v, _ := cmd.Flags().GetString("memory"); v != "" {
		opts["memory"] = v
	}
	if v, _ := cmd.Flags().GetFloat64("cpus"); v > 0 {
		opts["cpus"] = v
	}
	if v, _ := cmd.Flags().GetInt64("cpu-shares"); v > 0 {
		opts["cpu_shares"] = v
	}
	if v, _ := cmd.Flags().GetInt64("pids-limit"); v > 0 {
		opts["pids_limit"] = v
	}
	if v, _ := cmd.Flags().GetString("restart"); v != "" {
		opts["restart"] = v
	}
	return opts
}
