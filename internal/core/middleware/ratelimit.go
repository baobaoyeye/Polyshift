package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/polyshift/microkernel/internal/core/config"
	"golang.org/x/time/rate"
)

func RateLimit(cfg config.RateLimitConfig) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	// Create a global limiter
	// QPS: events per second
	// Burst: max events in a burst
	limiter := rate.NewLimiter(rate.Limit(cfg.QPS), cfg.Burst)

	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests",
			})
			return
		}
		c.Next()
	}
}
