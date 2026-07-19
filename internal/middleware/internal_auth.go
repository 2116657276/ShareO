package middleware

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// InternalTokenAuth validates the X-Internal-Token header for ai-service communication.
// Returns 503 if internal token is not configured, 401 if token doesn't match.
func InternalTokenAuth() gin.HandlerFunc {
	token := os.Getenv("SHAREO_INTERNAL_TOKEN")
	return func(c *gin.Context) {
		if token == "" {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		if c.GetHeader("X-Internal-Token") != token {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	}
}
