//go:build linux

package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/checkpoint"
)

// CheckpointCmd manages container checkpoints (requires CRIU).
func CheckpointCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checkpoint",
		Short: "Manage container checkpoints (requires CRIU)",
	}

	create := &cobra.Command{
		Use: "create [container] [name]", Short: "Checkpoint a running container", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cp, err := checkpoint.Create(args[0], args[1])
			if err != nil {
				return err
			}
			fmt.Printf("Checkpointed %s as %s\n", cp.ContainerID, cp.Name)
			return nil
		},
	}

	ls := &cobra.Command{
		Use: "ls [container]", Short: "List checkpoints", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cps, err := checkpoint.List(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("%-20s %s\n", "NAME", "CREATED")
			for _, cp := range cps {
				fmt.Printf("%-20s %s\n", cp.Name, cp.Created.Format("2006-01-02 15:04"))
			}
			return nil
		},
	}

	rm := &cobra.Command{
		Use: "rm [container] [name]", Short: "Remove a checkpoint", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return checkpoint.Remove(args[0], args[1])
		},
	}

	cmd.AddCommand(create, ls, rm)
	return cmd
}

// restoreCheckpoint restores a container; used by start --checkpoint.
func restoreCheckpoint(ctx context.Context, containerID, name string) error {
	return checkpoint.Restore(ctx, containerID, name)
}
