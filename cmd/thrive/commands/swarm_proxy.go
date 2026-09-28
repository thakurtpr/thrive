//go:build !linux

package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

// readComposeSpec reads a compose file for daemon-side deploys.
func readComposeSpec(file string) (string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("compose: cannot read %s: %w", file, err)
	}
	return string(data), nil
}

func dialSwarm(cmd *cobra.Command, bridge string, args []string, opts map[string]any) ([]byte, error) {
	data, err := vm.DialControl(cmd.Context(), bridge, args, opts)
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w", bridge, err)
	}
	return data, nil
}

func printResultJSON(data []byte) {
	var result any
	if err := json.Unmarshal(data, &result); err == nil {
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))
		return
	}
	fmt.Print(string(data))
}

// SwarmCmd proxies swarm mode to the VM daemon.
func SwarmCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "swarm", Short: "Manage single-node swarm mode"}

	initCmd := &cobra.Command{
		Use: "init", Short: "Initialise swarm mode on this node",
		RunE: func(cmd *cobra.Command, args []string) error {
			addr, _ := cmd.Flags().GetString("advertise-addr")
			data, err := dialSwarm(cmd, "swarm-init", nil, map[string]any{"addr": addr})
			if err != nil {
				return err
			}
			printResultJSON(data)
			return nil
		},
	}
	initCmd.Flags().String("advertise-addr", "", "Advertised manager address")

	join := &cobra.Command{
		Use: "join [address]", Short: "Join a swarm (single-node only)", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			token, _ := cmd.Flags().GetString("token")
			_, err := dialSwarm(cmd, "swarm-join", args, map[string]any{"token": token})
			return err
		},
	}
	join.Flags().String("token", "", "Join token")

	leave := &cobra.Command{
		Use: "leave", Short: "Leave swarm mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			_, err := dialSwarm(cmd, "swarm-leave", nil, map[string]any{"force": force})
			return err
		},
	}
	leave.Flags().BoolP("force", "f", false, "Leave despite running services")

	inspect := &cobra.Command{
		Use: "inspect", Short: "Inspect swarm state",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := dialSwarm(cmd, "swarm-inspect", nil, nil)
			if err != nil {
				return err
			}
			printResultJSON(data)
			return nil
		},
	}

	token := &cobra.Command{
		Use: "join-token [worker|manager]", Short: "Show or rotate a join token", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rotate, _ := cmd.Flags().GetBool("rotate")
			data, err := dialSwarm(cmd, "swarm-token", args, map[string]any{"rotate": rotate})
			if err != nil {
				return err
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			fmt.Println(result["token"])
			return nil
		},
	}
	token.Flags().Bool("rotate", false, "Rotate the token")

	cmd.AddCommand(initCmd, join, leave, inspect, token)
	return cmd
}

// ServiceCmd proxies service management to the VM daemon.
func ServiceCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "service", Short: "Manage replicated services"}

	create := &cobra.Command{
		Use: "create [image] [command...]", Short: "Create a replicated service", Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := dialSwarm(cmd, "service-create", args, serviceCreateOpts(cmd))
			return err
		},
	}
	create.Flags().String("name", "", "Service name (required)")
	create.Flags().IntP("replicas", "r", 1, "Replica count")
	create.Flags().StringArrayP("env", "e", nil, "Environment variables")
	create.Flags().StringArray("secret", nil, "Secrets to inject")
	create.Flags().StringArray("config", nil, "Config object mount: name:/container/path")
	create.Flags().StringArrayP("publish", "p", nil, "Publish ports")
	create.Flags().StringArrayP("volume", "v", nil, "Volumes")
	create.Flags().String("network", "", "Network mode or network name")
	create.Flags().Int("update-parallelism", 1, "Rolling update batch size")
	create.Flags().Int("update-delay", 0, "Seconds between update batches")
	create.Flags().String("restart", "no", "Restart policy")
	create.Flags().SetInterspersed(false)

	simple := func(use, short, bridge string, minArgs, maxArgs int) *cobra.Command {
		return &cobra.Command{
			Use: use, Short: short, Args: cobra.RangeArgs(minArgs, maxArgs),
			RunE: func(cmd *cobra.Command, args []string) error {
				data, err := dialSwarm(cmd, bridge, args, nil)
				if err != nil {
					return err
				}
				printResultJSON(data)
				return nil
			},
		}
	}

	scale := &cobra.Command{
		Use: "scale [service=num...]", Short: "Scale services", Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := dialSwarm(cmd, "service-scale", args, nil)
			return err
		},
	}

	update := &cobra.Command{
		Use: "update [service]", Short: "Rolling-update a service image", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			newImage, _ := cmd.Flags().GetString("image")
			_, err := dialSwarm(cmd, "service-update", args, map[string]any{"image": newImage})
			return err
		},
	}
	update.Flags().String("image", "", "New image (required)")

	cmd.AddCommand(create,
		simple("ls", "List services", "service-ls", 0, 0),
		simple("inspect [service]", "Display service details", "service-inspect", 1, 1),
		simple("ps [service]", "List service tasks", "service-ps", 1, 1),
		simple("rm [service]", "Remove a service", "service-rm", 1, 1),
		scale,
		simple("logs [service]", "Show service task logs", "service-logs", 1, 1),
		update,
		simple("rollback [service]", "Roll back a service", "service-rollback", 1, 1),
	)
	return cmd
}

// StackCmd proxies stack management to the VM daemon.
func StackCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{Use: "stack", Short: "Manage stacks (compose applications on swarm)"}

	deploy := &cobra.Command{
		Use: "deploy [stack]", Short: "Deploy a stack from a compose file", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := readComposeSpec(file)
			if err != nil {
				return err
			}
			_, err = dialSwarm(cmd, "stack-deploy", args, map[string]any{"spec": spec})
			return err
		},
	}
	deploy.Flags().StringVarP(&file, "file", "f", "docker-compose.yml", "Compose file path")

	simple := func(use, short, bridge string, n int) *cobra.Command {
		return &cobra.Command{
			Use: use, Short: short, Args: cobra.ExactArgs(n),
			RunE: func(cmd *cobra.Command, args []string) error {
				data, err := dialSwarm(cmd, bridge, args, nil)
				if err != nil {
					return err
				}
				printResultJSON(data)
				return nil
			},
		}
	}

	cmd.AddCommand(deploy,
		simple("ls", "List stacks", "stack-ls", 0),
		simple("ps [stack]", "List stack tasks", "stack-ps", 1),
		simple("services [stack]", "List stack services", "stack-services", 1),
		simple("rm [stack]", "Remove a stack", "stack-rm", 1),
	)
	return cmd
}
