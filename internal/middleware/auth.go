package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/zhoujianlin/ShareO/internal/pkg/jwt"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

// isWebRequest returns true if the request is a browser page visit (not an API call).
func isWebRequest(c *gin.Context) bool {
	return !strings.HasPrefix(c.Request.URL.Path, "/api/")
}

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// Check cookie for web pages
			token, err := c.Cookie("token")
			if err != nil || token == "" {
				if isWebRequest(c) {
					c.Redirect(http.StatusFound, "/login")
				} else {
					response.Unauthorized(c, "请先登录")
				}
				c.Abort()
				return
			}
			authHeader = "Bearer " + token
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := jwt.ParseToken(token)
		if err != nil {
			if isWebRequest(c) {
				// Clear invalid cookie and redirect to login
				http.SetCookie(c.Writer, &http.Cookie{
					Name: "token", Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
				})
				c.Redirect(http.StatusFound, "/login")
			} else {
				response.Unauthorized(c, "token无效或已过期")
			}
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		// Redis login state check: verify token is still cached, refresh TTL
		cachedToken, err := repository.GetLoginToken(context.Background(), claims.UserID)
		if err != nil {
			if err == redis.Nil {
				// Cache expired — redirect web pages to login, return JSON for API
				if isWebRequest(c) {
					http.SetCookie(c.Writer, &http.Cookie{
						Name: "token", Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
					})
					c.Redirect(http.StatusFound, "/login")
				} else {
					response.Unauthorized(c, "登录已过期，请重新登录")
				}
				c.Abort()
				return
			}
			// Redis error — fail-open (consistent with rate limiter behavior)
			slog.Warn("Redis error checking login cache", "user_id", claims.UserID, "err", err)
		} else {
			if cachedToken != token {
				// Token mismatch — logged in from another device/session
				if isWebRequest(c) {
					http.SetCookie(c.Writer, &http.Cookie{
						Name: "token", Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
					})
					c.Redirect(http.StatusFound, "/login")
				} else {
					response.Unauthorized(c, "账号已在其他设备登录，请重新登录")
				}
				c.Abort()
				return
			}
			// Token matches — refresh TTL (sliding window)
			if err := repository.RefreshLoginToken(context.Background(), claims.UserID, repository.LoginCacheTTL); err != nil {
				slog.Warn("failed to refresh login TTL", "user_id", claims.UserID, "err", err)
			}
		}

		c.Next()
	}
}

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role.(string) != "admin" {
			if isWebRequest(c) {
				c.Redirect(http.StatusFound, "/")
			} else {
				response.Forbidden(c, "需要管理员权限")
			}
			c.Abort()
			return
		}
		c.Next()
	}
}

// OptionalAuth tries to parse auth but doesn't block
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			token, _ := c.Cookie("token")
			if token != "" {
				authHeader = "Bearer " + token
			}
		}

		if authHeader == "" {
			c.Set("user_id", int64(0))
			c.Set("username", "")
			c.Set("role", "")
			c.Next()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := jwt.ParseToken(token)
		if err != nil {
			c.Set("user_id", int64(0))
			c.Next()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// RedirectIfAuth redirects to home if already logged in (for login/register pages)
func RedirectIfAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("token")
		if err != nil || token == "" {
			c.Next()
			return
		}
		claims, err := jwt.ParseToken(token)
		if err == nil && claims.UserID > 0 {
			c.Redirect(http.StatusFound, "/")
			c.Abort()
			return
		}
		c.Next()
	}
}
