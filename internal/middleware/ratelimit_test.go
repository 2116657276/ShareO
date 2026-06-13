package middleware

import (
	"testing"
	"time"
)

func TestRateLimit_ConfigValidation(t *testing.T) {
	// Verify that the RateLimit factory function accepts reasonable parameters
	// without panicking (we can't easily test Redis-dependent behavior here).
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
