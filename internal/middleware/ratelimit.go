package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

// setExpireScript atomically increments a key and sets expiry on first creation.
var setExpireScript = redis.NewScript(`
	local c = redis.call('INCR', KEYS[1])
	if c == 1 then
		redis.call('EXPIRE', KEYS[1], ARGV[1])
	end
	return c
`)

// RateLimit returns a middleware that limits requests per IP+endpoint using Redis.
// maxRequests: max requests allowed within the window
// window: fixed window (INCR + EXPIRE)
func RateLimit(maxRequests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := "ratelimit:" + ip + ":" + c.Request.URL.Path

		// Fail-open when Redis is not initialized or unavailable
		if repository.RDB == nil {
			slog.Warn("Redis not initialized, rate limiting disabled (fail-open)")
			c.Next()
			return
		}

		ctx := c.Request.Context()
		count, err := setExpireScript.Run(ctx, repository.RDB, []string{key}, int(window.Seconds())).Int64()
		if err != nil {
			// Redis error: allow request to pass through (fail-open)
			slog.Warn("Redis error in rate limiter (fail-open)", "err", err)
			c.Next()
			return
		}

		if count > int64(maxRequests) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    response.ErrCodeRateLimit,
				"message": "请求过于频繁，请稍后再试",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
