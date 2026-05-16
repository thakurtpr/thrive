//go:build !linux

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func MetricsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "metrics",
		Short: "Start metrics server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("metrics: runs inside the Thrive VM on Linux — use `thrive desktop start` and connect to :9090/metrics from the VM")
		},
	}
}
