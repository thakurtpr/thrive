// cmd/thrive/commands/desktop.go

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thakurprasadrout/thrive/internal/vm"
)

func DesktopCmd() *cobra.Command {
	desktop := &cobra.Command{
		Use:   "desktop",
		Short: "Manage Thrive Desktop VM",
		Long:  "Initialize, start, stop, or check status of the Thrive Desktop VM",
	}

	desktop.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Initialize Thrive Desktop (download VM image, create config)",
		RunE:  runDesktopInit,
	})

	desktop.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "Start the Thrive Desktop VM",
		RunE:  runDesktopStart,
	})

	desktop.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "Stop the Thrive Desktop VM",
		RunE:  runDesktopStop,
	})

	desktop.AddCommand(&cobra.Command{
		Use:   "restart",
		Short: "Restart the Thrive Desktop VM",
		RunE:  runDesktopRestart,
	})

	desktop.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show Thrive Desktop VM status",
		RunE:  runDesktopStatus,
	})

	return desktop
}

func runDesktopInit(cmd *cobra.Command, args []string) error {
	vmType := vm.DetectVMType()
	if vmType == "" {
		return fmt.Errorf("vm initialization not supported on this platform")
	}

	if err := vm.DownloadVMImage(cmd.Context()); err != nil {
		return fmt.Errorf("failed to download VM image: %w", err)
	}

	cfg := &vm.Config{
		MemoryMB: 2048,
		CPUCount: 2,
		VMType:   vmType,
	}
	if err := vm.WriteConfig(cfg); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	state := &vm.VMState{
		Version: "1.0",
		Running: false,
	}
	if err := vm.WriteVMState(state); err != nil {
		return fmt.Errorf("failed to write vm state: %w", err)
	}

	fmt.Println("Thrive Desktop initialized.")
	fmt.Printf("  VM type: %s\n", vmType)
	fmt.Printf("  Memory: %d MB\n", cfg.MemoryMB)
	fmt.Printf("  CPUs: %d\n", cfg.CPUCount)
	fmt.Println("\nRun `thrive desktop start` to begin.")

	return nil
}

func runDesktopStart(cmd *cobra.Command, args []string) error {
	cfg, err := vm.ReadConfig()
	if err != nil {
		return fmt.Errorf("run `thrive desktop init` first: %w", err)
	}

	if err := vm.Start(cmd.Context(), cfg); err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}

	if err := vm.WaitForBoot(cmd.Context(), cfg); err != nil {
		return fmt.Errorf("VM failed to boot: %w", err)
	}

	state, _ := vm.ReadVMState()
	state.Running = true
	vm.WriteVMState(state)

	fmt.Println("Thrive Desktop VM started.")
	return nil
}

func runDesktopStop(cmd *cobra.Command, args []string) error {
	if err := vm.Stop(cmd.Context()); err != nil {
		return fmt.Errorf("failed to stop VM: %w", err)
	}

	state, _ := vm.ReadVMState()
	state.Running = false
	vm.WriteVMState(state)

	fmt.Println("Thrive Desktop VM stopped.")
	return nil
}

func runDesktopRestart(cmd *cobra.Command, args []string) error {
	if err := vm.Stop(cmd.Context()); err != nil {
		fmt.Printf("warning: stop failed: %v\n", err)
	}

	cfg, err := vm.ReadConfig()
	if err != nil {
		return fmt.Errorf("run `thrive desktop init` first: %w", err)
	}

	if err := vm.Start(cmd.Context(), cfg); err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}

	if err := vm.WaitForBoot(cmd.Context(), cfg); err != nil {
		return fmt.Errorf("VM failed to boot: %w", err)
	}

	state, _ := vm.ReadVMState()
	state.Running = true
	vm.WriteVMState(state)

	fmt.Println("Thrive Desktop VM restarted.")
	return nil
}

func runDesktopStatus(cmd *cobra.Command, args []string) error {
	state, err := vm.ReadVMState()
	if err != nil {
		return fmt.Errorf("run `thrive desktop init` first: %w", err)
	}

	cfg, _ := vm.ReadConfig()

	fmt.Println("Thrive Desktop VM Status")
	fmt.Printf("  Running: %v\n", state.Running)
	if state.Running {
		fmt.Printf("  PID: %d\n", state.PID)
	}
	fmt.Printf("  VM type: %s\n", cfg.VMType)
	fmt.Printf("  Memory: %d MB\n", cfg.MemoryMB)
	fmt.Printf("  CPUs: %d\n", cfg.CPUCount)

	return nil
}