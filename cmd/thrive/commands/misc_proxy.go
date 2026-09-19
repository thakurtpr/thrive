//go:build !linux

package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

// AttachCmd streams container output via the daemon. Stdin forwarding opens
// an exec session (container stdio is detached at start).
func AttachCmd() *cobra.Command {
	var noStdin bool
	cmd := &cobra.Command{
		Use:   "attach [container]",
		Short: "Attach to a running container's output (stdin opens an exec session)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !noStdin {
				fmt.Fprintf(os.Stderr, "attach: streaming logs; stdin opens an exec shell\n")
				go func() {
					_ = vm.DialControlStream(cmd.Context(), "logs", args, map[string]any{"follow": true}, os.Stdout)
				}()
			}
			_, err := vm.DialControl(cmd.Context(), "attach", args, map[string]any{"no-stdin": noStdin})
			return err
		},
	}
	cmd.Flags().BoolVar(&noStdin, "no-stdin", false, "Do not forward stdin (output only)")
	return cmd
}

// VersionCmd prints client and server versions.
func VersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show client and server version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmd.Root().Version
			if client == "" {
				client = "dev"
			}
			fmt.Printf("Client:\n  Version: %s\n", client)
			data, err := vm.DialControl(cmd.Context(), "version", nil, nil)
			if err != nil {
				fmt.Printf("Server:\n  unreachable: %v\n", err)
				return nil
			}
			var result map[string]any
			json.Unmarshal(data, &result)
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Printf("Server:\n%s\n", string(out))
			return nil
		},
	}
}

// NodeCmd proxies single-node management to the daemon.
func NodeCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "node", Short: "Manage swarm nodes (single-node)"}

	simple := func(use, short, bridge string, n int) *cobra.Command {
		return &cobra.Command{
			Use: use, Short: short, Args: cobra.MaximumNArgs(n),
			RunE: func(cmd *cobra.Command, args []string) error {
				data, err := vm.DialControl(cmd.Context(), bridge, args, nil)
				if err != nil {
					return fmt.Errorf("%s failed: %w", bridge, err)
				}
				printResultJSON(data)
				return nil
			},
		}
	}

	cmd.AddCommand(
		simple("ls", "List nodes", "swarm-node-ls", 0),
		simple("inspect [node]", "Inspect the node", "swarm-node-inspect", 1),
		simple("promote [node]", "Promote a node", "swarm-node-promote", 1),
		simple("demote [node]", "Demote a node", "swarm-node-demote", 1),
		simple("ps [node]", "List tasks on the node", "swarm-node-ps", 1),
	)
	return cmd
}

// ConfigCmd proxies config objects to the daemon.
func ConfigCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Manage swarm config objects"}

	create := &cobra.Command{
		Use: "create [name] [file|-]", Short: "Create a config object", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var spec string
			if args[1] == "-" {
				raw, err := readAllStdin()
				if err != nil {
					return err
				}
				spec = string(raw)
			} else {
				var err error
				spec, err = readComposeSpec(args[1])
				if err != nil {
					return err
				}
			}
			_, err := vm.DialControl(cmd.Context(), "config-create", []string{args[0]}, map[string]any{"data": spec})
			return err
		},
	}

	simple := func(use, short, bridge string, n int) *cobra.Command {
		return &cobra.Command{
			Use: use, Short: short, Args: cobra.ExactArgs(n),
			RunE: func(cmd *cobra.Command, args []string) error {
				data, err := vm.DialControl(cmd.Context(), bridge, args, nil)
				if err != nil {
					return fmt.Errorf("%s failed: %w", bridge, err)
				}
				printResultJSON(data)
				return nil
			},
		}
	}

	cmd.AddCommand(create,
		simple("ls", "List configs", "config-ls", 0),
		simple("inspect [config]", "Inspect a config object", "config-inspect", 1),
		simple("rm [config]", "Remove a config object", "config-rm", 1),
	)
	return cmd
}

// BuilderCmd proxies build-cache operations to the daemon.
func BuilderCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "builder", Short: "Manage build cache"}
	du := &cobra.Command{
		Use: "du", Short: "Show build cache usage",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "buildx-du", nil, nil)
			if err != nil {
				return err
			}
			printResultJSON(data)
			return nil
		},
	}
	prune := &cobra.Command{
		Use: "prune", Short: "Clear the build cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "buildx-prune", nil, nil)
			if err != nil {
				return err
			}
			printResultJSON(data)
			return nil
		},
	}
	cmd.AddCommand(du, prune)
	return cmd
}

// ImageCmd groups image read operations (native proxy constructors).
func ImageCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "image", Short: "Manage images"}
	prune := &cobra.Command{
		Use: "prune", Short: "Remove unreferenced images",
		RunE: func(cmd *cobra.Command, args []string) error {
			all, _ := cmd.Flags().GetBool("all")
			data, err := vm.DialControl(cmd.Context(), "image-prune", nil, map[string]any{"all": all})
			if err != nil {
				return err
			}
			printResultJSON(data)
			return nil
		},
	}
	prune.Flags().BoolP("all", "a", false, "Remove all unreferenced images")
	prune.Flags().BoolP("force", "f", false, "Do not prompt (no prompt currently required)")
	cmd.AddCommand(ImagesCmd(), HistoryCmd(), prune)
	return cmd
}

// ContainerCmd groups container read/prune operations.
func ContainerCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "container", Short: "Manage containers"}
	prune := &cobra.Command{
		Use: "prune", Short: "Remove stopped containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := vm.DialControl(cmd.Context(), "container-prune", nil, nil)
			if err != nil {
				return err
			}
			printResultJSON(data)
			return nil
		},
	}
	cmd.AddCommand(PsCmd(), LogsCmd(), prune)
	return cmd
}
