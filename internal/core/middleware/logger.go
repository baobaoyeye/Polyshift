package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()
		requestID := c.GetString("RequestID")

		if raw != "" {
			path = path + "?" + raw
		}

		logger := slog.Default()
		
		attrs := []any{
			slog.String("method", method),
			slog.String("path", path),
			slog.Int("status", status),
			slog.String("latency", latency.String()),
			slog.String("client_ip", clientIP),
		}
		
		if requestID != "" {
			attrs = append(attrs, slog.String("request_id", requestID))
		}
		
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()
		if errorMessage != "" {
			attrs = append(attrs, slog.String("error", errorMessage))
			logger.Error("Request failed", attrs...)
		} else {
			if status >= 500 {
				logger.Error("Server error", attrs...)
			} else if status >= 400 {
				logger.Warn("Client error", attrs...)
			} else {
				logger.Info("Request success", attrs...)
			}
		}
	}
}
