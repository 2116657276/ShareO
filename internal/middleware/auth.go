package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/pkg/jwt"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// Check cookie for web pages
			token, err := c.Cookie("token")
			if err != nil || token == "" {
				response.Unauthorized(c, "请先登录")
				c.Abort()
				return
			}
			authHeader = "Bearer " + token
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := jwt.ParseToken(token)
		if err != nil {
			response.Unauthorized(c, "token无效或已过期")
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
			response.Forbidden(c, "需要管理员权限")
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
