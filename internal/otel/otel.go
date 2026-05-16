//go:build linux
// +build linux

package otel

import (
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/thakurprasadrout/thrive/internal/telemetry"
)

type Config struct {
	ServiceName    string
	MetricsEnabled bool
	TracesEnabled  bool
	OTLPEndpoint   string
	PrometheusAddr string
}

var (
	promRegistry   = prometheus.NewRegistry()
	containerCount prometheus.Counter
	cpuUsage       prometheus.Counter
	memUsage       prometheus.Gauge
)

func Init(cfg Config) error {
	log := telemetry.Logger()
	log.Info("otel.Init: starting", telemetry.FieldString("serviceName", cfg.ServiceName))

	if cfg.ServiceName == "" {
		cfg.ServiceName = "thrive"
	}

	initMetricsInstruments()

	log.Info("otel.Init: completed")
	return nil
}

func initMetricsInstruments() {
	log := telemetry.Logger()
	log.Debug("otel.initMetricsInstruments: creating instruments")

	containerCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "thrive_container_count",
		Help: "Number of containers",
	})
	cpuUsage = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "thrive_container_cpu_seconds_total",
		Help: "Container CPU usage in seconds",
	})
	memUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "thrive_container_memory_bytes",
		Help: "Container memory usage in bytes",
	})

	promRegistry.MustRegister(containerCount, cpuUsage, memUsage)
	log.Debug("otel.initMetricsInstruments: instruments created")
}

func RecordContainerCPU(containerID string, nanos float64) {
	if cpuUsage != nil {
		cpuUsage.Add(nanos / 1e9)
	}
}

func RecordContainerMemory(containerID string, bytes float64) {
	if memUsage != nil {
		memUsage.Set(bytes)
	}
}

func IncContainerCount(containerID string) {
	if containerCount != nil {
		containerCount.Inc()
	}
}

func DecContainerCount(containerID string) {
	if containerCount != nil {
		containerCount.Add(-1)
	}
}

func MetricsHandler() http.Handler {
	return promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{})
}

func MetricsAddr() string {
	return os.Getenv("THRIVE_METRICS_ADDR")
}

func StartMetricsServer(addr string) error {
	log := telemetry.Logger()
	log.Info("otel.StartMetricsServer: starting", telemetry.FieldString("addr", addr))

	if addr == "" {
		addr = ":9090"
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{}))

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Info("otel.StartMetricsServer: listening", telemetry.FieldString("addr", addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("otel.StartMetricsServer: ListenAndServe failed", telemetry.FieldError(err))
		}
	}()

	return nil
}