//go:build linux

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/buildx"
	thrruntime "github.com/thakurprasadrout/thrive/internal/runtime"
	"github.com/thakurprasadrout/thrive/internal/swarm"
	"github.com/thakurprasadrout/thrive/internal/swarmconfig"
	thrsystem "github.com/thakurprasadrout/thrive/internal/system"
)

// AttachCmd attaches to a container's output, forwarding stdin to an exec
// session unless --no-stdin is given. Container stdio is detached at start,
// so output comes from the log stream; this is documented, not hidden.
func AttachCmd() *cobra.Command {
	var noStdin bool
	cmd := &cobra.Command{
		Use:   "attach [container]",
		Short: "Attach to a running container's output (stdin opens an exec session)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			id := args[0]
			state, err := thrruntime.State(ctx, id)
			if err != nil || state.Status != "running" || state.PID == 0 {
				fmt.Fprintf(os.Stderr, "Error: container %s is not running\n", id)
				os.Exit(1)
			}
			if !noStdin {
				fmt.Fprintf(os.Stderr, "attach: streaming logs; stdin opens an exec shell (container stdio is detached at start)\n")
				go streamLogFollow(ctx, id)
				nsenterArgs := []string{
					"--target", strconv.Itoa(state.PID),
					"--mount", "--pid", "--ipc", "--uts", "--net",
					"--", "/bin/sh",
				}
				os.Exit(execInContainerNS(ctx, nsenterArgs))
				return
			}
			streamLogFollow(ctx, id)
		},
	}
	cmd.Flags().BoolVar(&noStdin, "no-stdin", false, "Do not forward stdin (output only)")
	return cmd
}

func streamLogFollow(ctx context.Context, id string) {
	logPath := filepath.Join("/run/thrive/containers", id, "logs")
	f, err := os.Open(logPath)
	if err != nil {
		return
	}
	defer f.Close() //nolint:errcheck
	if _, err := io.Copy(os.Stdout, f); err != nil {
		return
	}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		if _, err := io.Copy(os.Stdout, f); err != nil {
			return
		}
		state, err := thrruntime.State(ctx, id)
		if err != nil || state.Status == "stopped" {
			return
		}
	}
}

// VersionCmd prints client and (daemonless) server versions.
func VersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show client and server version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := cmd.Root().Version
			if client == "" {
				client = "dev"
			}
			fmt.Printf("Client:\n  Version: %s\n  Go: %s\n  OS/Arch: %s/%s\n",
				client, runtime.Version(), runtime.GOOS, runtime.GOARCH)
			fmt.Printf("Server:\n  Engine: thrive (daemonless)\n  Version: %s\n  OS/Arch: %s/%s\n",
				client, runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}
}

// NodeCmd manages the single swarm node.
func NodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Manage swarm nodes (single-node)",
	}

	ls := &cobra.Command{
		Use: "ls", Short: "List nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := swarm.Inspect()
			if err != nil {
				return err
			}
			fmt.Printf("%-25s %-10s %s\n", "ID", "ROLE", "ADDR")
			fmt.Printf("%-25s %-10s %s\n", st.NodeID, "manager", st.ManagerAddr)
			return nil
		},
	}

	inspectNode := &cobra.Command{
		Use: "inspect [node]", Short: "Inspect the node", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := swarm.Inspect()
			if err != nil {
				return err
			}
			out, _ := json.MarshalIndent(map[string]any{
				"id": st.NodeID, "role": "manager", "addr": st.ManagerAddr,
			}, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	promote := &cobra.Command{
		Use: "promote [node]", Short: "Promote a node (already the manager on single-node)",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := swarm.RequireInitialised(); err != nil {
				return err
			}
			fmt.Println("Node is already a manager (single-node swarm)")
			return nil
		},
	}

	demote := &cobra.Command{
		Use: "demote [node]", Short: "Demote a node",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("node: cannot demote the last manager (single-node swarm)")
		},
	}

	psNode := &cobra.Command{
		Use: "ps [node]", Short: "List tasks running on the node", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := os.ReadDir("/run/thrive/containers")
			if err != nil {
				return err
			}
			fmt.Printf("%-35s %-10s %s\n", "ID", "STATUS", "PID")
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				state, err := thrruntime.State(context.Background(), e.Name())
				if err != nil {
					continue
				}
				fmt.Printf("%-35s %-10s %d\n", e.Name(), state.Status, state.PID)
			}
			return nil
		},
	}

	cmd.AddCommand(ls, inspectNode, promote, demote, psNode)
	return cmd
}

// ConfigCmd manages swarm config objects.
func ConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage swarm config objects",
	}

	create := &cobra.Command{
		Use: "create [name] [file|-]", Short: "Create a config object", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var data []byte
			var err error
			if args[1] == "-" {
				data, err = io.ReadAll(os.Stdin)
			} else {
				data, err = os.ReadFile(args[1])
			}
			if err != nil {
				return fmt.Errorf("config: read: %w", err)
			}
			cfg, err := swarmconfig.Create(args[0], data)
			if err != nil {
				return err
			}
			fmt.Printf("Created config %s (%d bytes)\n", cfg.Name, cfg.Size)
			return nil
		},
	}

	ls := &cobra.Command{
		Use: "ls", Short: "List configs",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgs, err := swarmconfig.List()
			if err != nil {
				return err
			}
			fmt.Printf("%-25s %-8s %s\n", "NAME", "SIZE", "CREATED")
			for _, c := range cfgs {
				fmt.Printf("%-25s %-8d %s\n", c.Name, c.Size, c.Created.Format("2006-01-02 15:04"))
			}
			return nil
		},
	}

	inspectCfg := &cobra.Command{
		Use: "inspect [config]", Short: "Inspect a config object", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := swarmconfig.Inspect(args[0])
			if err != nil {
				return err
			}
			out, _ := json.MarshalIndent(cfg, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	rm := &cobra.Command{
		Use: "rm [config]", Short: "Remove a config object", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return swarmconfig.Remove(args[0])
		},
	}

	cmd.AddCommand(create, ls, inspectCfg, rm)
	return cmd
}

// BuilderCmd aliases build-cache operations.
func BuilderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "builder",
		Short: "Manage build cache",
	}
	du := &cobra.Command{
		Use: "du", Short: "Show build cache usage",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Build cache: %s\n", humanBytes(buildx.CacheUsage("/var/lib/thrive/cache")))
			return nil
		},
	}
	prune := &cobra.Command{
		Use: "prune", Short: "Clear the build cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			reclaimed, err := buildx.PruneCache("/var/lib/thrive/cache")
			if err != nil {
				return err
			}
			fmt.Printf("Reclaimed %s\n", humanBytes(reclaimed))
			return nil
		},
	}
	cmd.AddCommand(du, prune)
	return cmd
}

// ImageCmd groups image read operations.
func ImageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Manage images",
	}
	prune := &cobra.Command{
		Use: "prune", Short: "Remove unreferenced images",
		RunE: func(cmd *cobra.Command, args []string) error {
			all, _ := cmd.Flags().GetBool("all")
			force, _ := cmd.Flags().GetBool("force")
			if !force && !all {
				return fmt.Errorf("image prune: nothing to do without --all (all local images carry refs)")
			}
			deleted, reclaimed, err := thrsystem.PruneImages()
			if err != nil {
				return err
			}
			fmt.Printf("Deleted images: %d\nReclaimed: %s\n", deleted, humanBytes(reclaimed))
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
	cmd := &cobra.Command{
		Use:   "container",
		Short: "Manage containers",
	}
	prune := &cobra.Command{
		Use: "prune", Short: "Remove stopped containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			deleted, reclaimed, err := thrsystem.PruneContainers()
			if err != nil {
				return err
			}
			fmt.Printf("Deleted containers: %d\nReclaimed: %s\n", deleted, humanBytes(reclaimed))
			return nil
		},
	}
	cmd.AddCommand(PsCmd(), LogsCmd(), prune)
	return cmd
}
