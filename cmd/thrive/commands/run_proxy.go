// cmd/thrive/commands/run_proxy.go

//go:build !linux

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func RunCmd() *cobra.Command {
	run := &cobra.Command{
		Use:   "run",
		Short: "Run a container",
		RunE:  runViaVMBridge,
	}

	run.Flags().BoolP("detach", "d", false, "Run container in background")
	run.Flags().BoolP("rm", "", false, "Remove container when it exits")
	run.Flags().StringArrayP("env", "e", nil, "Set environment variables")
	run.Flags().StringArrayP("secret", "", nil, "Pass secret to container")
	run.Flags().StringP("name", "", "", "Assign a name to the container")

	return run
}

func runViaVMBridge(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("run requires an image argument")
	}

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

	opts := map[string]any{
		"detach": cmd.Flag("detach").Value.String() == "true",
		"rm":     cmd.Flag("rm").Value.String() == "true",
	}

	if name := cmd.Flag("name").Value.String(); name != "" {
		opts["name"] = name
	}

	envVars, _ := cmd.Flags().GetStringArray("env")
	if len(envVars) > 0 {
		opts["env"] = envVars
	}

	secretNames, _ := cmd.Flags().GetStringArray("secret")
	if len(secretNames) > 0 {
		opts["secrets"] = secretNames
	}

	runArgs := args
	data, err := bridge.Exec(ctx, "run", runArgs, opts)
	if err != nil {
		return fmt.Errorf("container run failed: %w", err)
	}

	var result map[string]any
	json.Unmarshal(data, &result)

	if id, ok := result["container_id"].(string); ok {
		fmt.Printf("container %s\n", id)
	}

	return nil
}