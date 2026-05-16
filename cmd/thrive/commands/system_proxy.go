// cmd/thrive/commands/system_proxy.go

//go:build !linux

package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func SystemCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "system",
		Short: "Show Thrive system information",
		RunE:  systemViaVMBridge,
	}
}

func systemViaVMBridge(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	cfg, err := vm.ReadConfig()
	if err != nil {
		return fmt.Errorf("run `thrive desktop init` first: %w", err)
	}

	bridge, err := vm.Dial(ctx, cfg.VMType)
	if err != nil {
		return fmt.Errorf("failed to connect to VM: %w\nRun `thrive desktop start` first.", err)
	}
	defer bridge.Close()

	data, err := bridge.Exec(ctx, "system", nil, nil)
	if err != nil {
		return fmt.Errorf("system info failed: %w", err)
	}

	fmt.Print(string(data))
	return nil
}