package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/otel"
	"github.com/thakurprasadrout/thrive/internal/telemetry"
)

func MetricsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "metrics",
		Short: "Start metrics server",
		Run: func(cmd *cobra.Command, args []string) {
			if err := telemetry.Init(); err != nil {
				fmt.Fprintf(os.Stderr, "telemetry init: %v\n", err)
				os.Exit(1)
			}

			addr := os.Getenv("THRIVE_METRICS_ADDR")
			if addr == "" {
				addr = ":9090"
			}

			fmt.Printf("Starting metrics server on %s/metrics\n", addr)

			if err := otel.Init(otel.Config{
				ServiceName:    "thrive",
				MetricsEnabled: true,
				PrometheusAddr: addr,
			}); err != nil {
				fmt.Fprintf(os.Stderr, "otel init: %v\n", err)
				os.Exit(1)
			}

			if err := otel.StartMetricsServer(addr); err != nil {
				fmt.Fprintf(os.Stderr, "metrics server: %v\n", err)
				os.Exit(1)
			}

			select {}
		},
	}
}