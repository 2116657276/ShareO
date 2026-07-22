package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestInternalTokenAuth(t *testing.T) {
	t.Setenv("SHAREO_INTERNAL_TOKEN", "test-internal-token")
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(InternalTokenAuth())
	r.GET("/internal/test", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	tests := []struct {
		name   string
		token  string
		status int
	}{
		{name: "missing", status: http.StatusUnauthorized},
		{name: "wrong", token: "wrong", status: http.StatusUnauthorized},
		{name: "valid", token: "test-internal-token", status: http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/internal/test", nil)
			if tt.token != "" {
				req.Header.Set("X-Internal-Token", tt.token)
			}
			r.ServeHTTP(recorder, req)
			if recorder.Code != tt.status {
				t.Fatalf("status=%d want=%d", recorder.Code, tt.status)
			}
		})
	}

}
