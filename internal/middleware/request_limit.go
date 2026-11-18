package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const DefaultRequestBodyLimit int64 = 1 << 20

// RequestBodyLimit bounds non-upload request bodies before JSON/form binding.
// Upload endpoints keep their larger, format-specific limit in the handler.
func RequestBodyLimit(maxBytes int64, excludedPaths ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, path := range excludedPaths {
			if c.Request.URL.Path == path {
				c.Next()
				return
			}
		}
		if maxBytes <= 0 || c.Request.Body == nil || strings.EqualFold(c.Request.Method, http.MethodGet) ||
			strings.EqualFold(c.Request.Method, http.MethodHead) || strings.EqualFold(c.Request.Method, http.MethodOptions) {
			c.Next()
			return
		}
		if c.Request.ContentLength > maxBytes {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
