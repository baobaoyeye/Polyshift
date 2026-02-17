package observability

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/polyshift/microkernel/internal/core/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// InitMeter initializes the OpenTelemetry meter provider with Prometheus exporter.
// It also starts a separate HTTP server to expose metrics on the configured port.
// Returns a shutdown function.
func InitMeter(cfg config.ObservabilityConfig) (func(context.Context) error, error) {
	if !cfg.Metrics.Enabled {
		slog.Info("Metrics are disabled")
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName("microkernel-core"),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	mp := metric.NewMeterProvider(
		metric.WithReader(exporter),
		metric.WithResource(res),
	)

	otel.SetMeterProvider(mp)

	// Start metrics server
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Metrics.Port),
		Handler: mux,
	}

	go func() {
		slog.Info("Starting metrics server", "port", cfg.Metrics.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Metrics server failed", "error", err)
		}
	}()

	slog.Info("Metrics initialized", "port", cfg.Metrics.Port)

	return func(ctx context.Context) error {
		// Shutdown metrics server
		if err := server.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown metrics server: %w", err)
		}
		// Shutdown MeterProvider
		if err := mp.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown meter provider: %w", err)
		}
		return nil
	}, nil
}
