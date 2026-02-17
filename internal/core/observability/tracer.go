package observability

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/polyshift/microkernel/internal/core/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// InitTracer initializes the OpenTelemetry tracer provider based on configuration.
// It returns a shutdown function that should be called when the application exits.
func InitTracer(cfg config.ObservabilityConfig) (func(context.Context) error, error) {
	if !cfg.Tracing.Enabled {
		slog.Info("Tracing is disabled")
		return func(context.Context) error { return nil }, nil
	}

	var exporter sdktrace.SpanExporter
	var err error

	switch cfg.Tracing.Exporter {
	case "otlp":
		// Assumes OTLP collector is running on localhost:4317 by default
		// In a real scenario, this endpoint should also be configurable
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		exporter, err = otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure())
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
		slog.Info("Initialized OTLP trace exporter")
	case "stdout":
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
		}
		slog.Info("Initialized Stdout trace exporter")
	default:
		slog.Warn("Unknown trace exporter, defaulting to stdout", "exporter", cfg.Tracing.Exporter)
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
		}
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

	// Configure sampling strategy
	var sampler sdktrace.Sampler
	if cfg.Tracing.SamplingRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if cfg.Tracing.SamplingRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.Tracing.SamplingRate)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set global TracerProvider
	otel.SetTracerProvider(tp)

	// Set global Propagator (W3C Trace Context)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	slog.Info("Tracing initialized", "exporter", cfg.Tracing.Exporter, "sampling_rate", cfg.Tracing.SamplingRate)

	return tp.Shutdown, nil
}
