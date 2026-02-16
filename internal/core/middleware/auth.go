package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/polyshift/microkernel/internal/core/config"
)

func APIKeyAuth(cfg config.AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.Enabled {
			c.Next()
			return
		}

		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			apiKey = c.Query("api_key")
		}

		if apiKey != cfg.APIKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Next()
	}
}
