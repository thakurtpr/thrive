// cmd/thrive/commands/ps_proxy.go

//go:build !linux

package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func PsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ps",
		Short: "List running containers",
		RunE:  psViaVMBridge,
	}
}

func psViaVMBridge(cmd *cobra.Command, args []string) error {
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

	data, err := bridge.Exec(ctx, "ps", nil, nil)
	if err != nil {
		return fmt.Errorf("ps failed: %w", err)
	}

	var result map[string]any
	json.Unmarshal(data, &result)

	containers, ok := result["containers"].([]any)
	if !ok || len(containers) == 0 {
		fmt.Println("no containers running")
		return nil
	}

	fmt.Println("CONTAINER ID   IMAGE   STATUS   PID")
	fmt.Println("─────────────────────────────────────")
	for _, c := range containers {
		cm := c.(map[string]any)
		id := cm["id"].(string)[:12]
		image := cm["image"].(string)
		status := cm["status"].(string)
		pid := 0
		if pidFloat, ok := cm["pid"].(float64); ok {
			pid = int(pidFloat)
		}
		fmt.Printf("%-13s %-9s %-9s %d\n", id, image, status, pid)
	}

	return nil
}
