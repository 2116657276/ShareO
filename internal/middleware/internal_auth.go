package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// InternalTokenAuth validates the X-Internal-Token header for ai-service communication.
// Returns 503 if internal token is not configured, 401 if token doesn't match.
func InternalTokenAuth() gin.HandlerFunc {
	token := strings.TrimSpace(os.Getenv("SHAREO_INTERNAL_TOKEN"))
	configured := token != "" && !isKnownUnsafeInternalToken(token)
	return func(c *gin.Context) {
		if !configured {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		provided := c.GetHeader("X-Internal-Token")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	}
}

func isKnownUnsafeInternalToken(token string) bool {
	switch token {
	case "shareo-local-internal", "replace-with-a-random-value":
		return true
	default:
		return false
	}
}
