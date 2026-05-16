// cmd/thrive/commands/logs_proxy.go

//go:build !linux

package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func LogsCmd() *cobra.Command {
	logs := &cobra.Command{
		Use:   "logs",
		Short: "Fetch container logs",
		RunE:  logsViaVMBridge,
	}

	logs.Flags().BoolP("follow", "f", false, "Follow log output")

	return logs
}

func logsViaVMBridge(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("logs requires a container ID")
	}

	ctx := cmd.Context()
	containerID := args[0]
	follow, _ := cmd.Flags().GetBool("follow")

	cfg, err := vm.ReadConfig()
	if err != nil {
		return fmt.Errorf("run `thrive desktop init` first: %w", err)
	}

	bridge, err := vm.Dial(ctx, cfg.VMType)
	if err != nil {
		return fmt.Errorf("failed to connect to VM: %w\nRun `thrive desktop start` first.", err)
	}
	defer bridge.Close()

	opts := map[string]any{"follow": follow}

	if follow {
		return bridge.ExecStream(ctx, "logs", []string{containerID}, opts, os.Stdout)
	}

	data, err := bridge.Exec(ctx, "logs", []string{containerID}, opts)
	if err != nil {
		return fmt.Errorf("logs failed: %w", err)
	}

	fmt.Print(string(data))
	return nil
}