package middleware

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddlewares returns a list of Gin middlewares for OpenTelemetry tracing.
// It includes the standard OTel middleware and a custom middleware to inject TraceID into response headers.
func TracingMiddlewares(serviceName string) []gin.HandlerFunc {
	return []gin.HandlerFunc{
		otelgin.Middleware(serviceName),
		func(c *gin.Context) {
			span := trace.SpanFromContext(c.Request.Context())
			if span.SpanContext().IsValid() {
				c.Header("X-Trace-ID", span.SpanContext().TraceID().String())
			}
			c.Next()
		},
	}
}
