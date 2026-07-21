package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/zhoujianlin/ShareO/internal/pkg/jwt"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

var (
	ErrMissingToken   = errors.New("missing token")
	ErrInvalidToken   = errors.New("invalid token")
	ErrExpiredSession = errors.New("expired session")
	ErrReplacedToken  = errors.New("replaced token")
)

// AuthenticateRequest performs the same JWT and Redis-backed session checks
// for HTTP handlers and WebSocket handshakes.
func AuthenticateRequest(r *http.Request, refresh bool) (*jwt.Claims, string, error) {
	token := ""
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		token = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}
	if token == "" {
		if cookie, err := r.Cookie("token"); err == nil {
			token = cookie.Value
		}
	}
	if token == "" {
		return nil, "", ErrMissingToken
	}
	claims, err := jwt.ParseToken(token)
	if err != nil {
		return nil, "", ErrInvalidToken
	}
	cachedToken, err := repository.GetLoginToken(r.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, "", ErrExpiredSession
		}
		slog.Warn("Redis error checking login cache", "user_id", claims.UserID, "err", err)
		return claims, token, nil // Preserve the project's fail-open Redis policy.
	}
	if cachedToken != token {
		return nil, "", ErrReplacedToken
	}
	if refresh {
		if err := repository.RefreshLoginToken(r.Context(), claims.UserID, repository.LoginCacheTTL); err != nil {
			slog.Warn("failed to refresh login TTL", "user_id", claims.UserID, "err", err)
		}
	}
	return claims, token, nil
}

// isWebRequest returns true if the request is a browser page visit (not an API call).
func isWebRequest(c *gin.Context) bool {
	return !strings.HasPrefix(c.Request.URL.Path, "/api/")
}

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, _, err := AuthenticateRequest(c.Request, true)
		if err != nil {
			if isWebRequest(c) {
				http.SetCookie(c.Writer, &http.Cookie{
					Name: "token", Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
				})
				c.Redirect(http.StatusFound, "/login")
			} else {
				message := "token无效或已过期"
				if errors.Is(err, ErrMissingToken) {
					message = "请先登录"
				} else if errors.Is(err, ErrReplacedToken) {
					message = "账号已在其他设备登录，请重新登录"
				} else if errors.Is(err, ErrExpiredSession) {
					message = "登录已过期，请重新登录"
				}
				response.Unauthorized(c, message)
			}
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

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
