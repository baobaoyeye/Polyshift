package middleware

import (
	"context"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// MetricsMiddleware returns a Gin middleware that records HTTP request metrics.
func MetricsMiddleware() gin.HandlerFunc {
	meter := otel.Meter("microkernel-gateway")

	requestDuration, err := meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("Duration of HTTP requests in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		slog.Error("Failed to create request duration histogram", "error", err)
	}

	requestCount, err := meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		slog.Error("Failed to create request count counter", "error", err)
	}

	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		c.Next()

		duration := time.Since(start).Seconds()
		status := c.Writer.Status()

		attrs := metric.WithAttributes(
			attribute.String("method", c.Request.Method),
			attribute.String("path", path),
			attribute.Int("status", status),
		)

		if requestDuration != nil {
			requestDuration.Record(context.Background(), duration, attrs)
		}
		if requestCount != nil {
			requestCount.Add(context.Background(), 1, attrs)
		}
	}
}
