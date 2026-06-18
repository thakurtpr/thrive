//go:build !linux

package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func RunCmd() *cobra.Command {
	run := &cobra.Command{
		Use:   "run",
		Short: "Run a container",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			opts := map[string]any{
				"detach": cmd.Flag("detach").Value.String() == "true",
				"rm":     cmd.Flag("rm").Value.String() == "true",
			}

			if name := cmd.Flag("name").Value.String(); name != "" {
				opts["name"] = name
			}

			if envVars, _ := cmd.Flags().GetStringArray("env"); len(envVars) > 0 {
				opts["env"] = envVars
			}

			if secrets, _ := cmd.Flags().GetStringArray("secret"); len(secrets) > 0 {
				opts["secrets"] = secrets
			}

			if ports, _ := cmd.Flags().GetStringArray("publish"); len(ports) > 0 {
				opts["ports"] = parseProxyPortSpecs(ports)
			}

			if volumes, _ := cmd.Flags().GetStringArray("volume"); len(volumes) > 0 {
				opts["volumes"] = volumes
			}

			if netMode := cmd.Flag("network").Value.String(); netMode != "" {
				opts["network"] = netMode
			}

			data, err := vm.DialControl(ctx, "run", args, opts)
			if err != nil {
				return fmt.Errorf("container run failed: %w", err)
			}

			var result map[string]any
			json.Unmarshal(data, &result)

			if id, ok := result["container_id"].(string); ok {
				fmt.Printf("container %s\n", id)
			}

			return nil
		},
	}

	run.Flags().BoolP("detach", "d", false, "Run container in background")
	run.Flags().Bool("rm", false, "Remove container when it exits")
	run.Flags().StringArrayP("env", "e", nil, "Set environment variables")
	run.Flags().StringArray("secret", nil, "Pass secret to container")
	run.Flags().String("name", "", "Assign a name to the container")
	run.Flags().StringArrayP("publish", "p", nil, "Publish port(s): host:container[/proto]")
	run.Flags().StringArrayP("volume", "v", nil, "Bind mount: /host:/container")
	run.Flags().String("network", "", "Network mode")

	return run
}

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
