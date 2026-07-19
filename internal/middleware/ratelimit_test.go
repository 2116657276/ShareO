package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func TestRateLimit_ConfigValidation(t *testing.T) {
	// Verify that the RateLimit factory function accepts reasonable parameters
	// without panicking.
	fn := RateLimit(10, 1*time.Minute)
	if fn == nil {
		t.Error("RateLimit should return a handler function, got nil")
	}
}

func TestRateLimit_DifferentConfigs(t *testing.T) {
	configs := []struct {
		maxRequests int
		window      time.Duration
	}{
		{10, 1 * time.Minute},
		{30, 1 * time.Minute},
		{20, 1 * time.Minute},
		{100, 1 * time.Hour},
		{1, 1 * time.Second},
	}

	for _, c := range configs {
		fn := RateLimit(c.maxRequests, c.window)
		if fn == nil {
			t.Errorf("RateLimit(%d, %v) returned nil", c.maxRequests, c.window)
		}
	}
}

func TestRateLimit_FailOpen(t *testing.T) {
	// When Redis is unavailable (RDB == nil), the middleware should
	// fail-open and allow the request to pass through.
	handler := RateLimit(10, 1*time.Minute)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/test", nil)

	// The handler should not panic and should call c.Next()
	handler(c)

	// Since Redis is not initialized in tests, the middleware should
	// log the error and pass through (fail-open). The response should
	// not be a 429.
	if w.Code == http.StatusTooManyRequests {
		t.Error("expected fail-open (not 429) when Redis is unavailable")
	}
}

func TestRateLimit_HandlerNonNil(t *testing.T) {
	handler := RateLimit(10, 1*time.Minute)
	if handler == nil {
		t.Fatal("RateLimit returned nil handler")
	}

	// Verify the handler can be invoked without panic
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/test", nil)

	// Should not panic
	handler(c)
}
