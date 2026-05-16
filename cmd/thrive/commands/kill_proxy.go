// cmd/thrive/commands/kill_proxy.go

//go:build !linux

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func KillCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kill",
		Short: "Kill a running container",
		Args:  cobra.ExactArgs(1),
		RunE:  killViaVMBridge,
	}
}

func killViaVMBridge(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	containerID := args[0]

	cfg, err := vm.ReadConfig()
	if err != nil {
		return fmt.Errorf("run `thrive desktop init` first: %w", err)
	}

	bridge, err := vm.Dial(ctx, cfg.VMType)
	if err != nil {
		return fmt.Errorf("failed to connect to VM: %w\nRun `thrive desktop start` first.", err)
	}
	defer bridge.Close()

	_, err = bridge.Exec(ctx, "kill", []string{containerID}, nil)
	if err != nil {
		return fmt.Errorf("kill failed: %w", err)
	}

	fmt.Printf("container %s killed\n", containerID)
	return nil
}