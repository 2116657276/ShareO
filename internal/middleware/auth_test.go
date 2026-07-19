package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/pkg/jwt"
)

func init() {
	jwt.Init("test-secret-for-middleware-tests", 72)
	gin.SetMode(gin.TestMode)
}

func TestAuthRequired_NoToken(t *testing.T) {
	// API path returns JSON 401
	r := gin.New()
	r.Use(AuthRequired())
	r.GET("/api/v1/test", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthRequired_NoToken_WebRedirect(t *testing.T) {
	// Web path redirects to /login
	r := gin.New()
	r.Use(AuthRequired())
	r.GET("/test", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected Location /login, got %s", loc)
	}
}

func TestAuthRequired_InvalidToken(t *testing.T) {
	// API path with invalid token returns JSON 401
	r := gin.New()
	r.Use(AuthRequired())
	r.GET("/api/v1/test", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthRequired_InvalidToken_WebRedirect(t *testing.T) {
	// Web path with invalid cookie token redirects to /login
	r := gin.New()
	r.Use(AuthRequired())
	r.GET("/home", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/home", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: "invalid-token"})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected Location /login, got %s", loc)
	}
}

func TestAuthRequired_ValidToken(t *testing.T) {
	token, err := jwt.GenerateToken(1, "testuser", "user")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	r := gin.New()
	r.Use(AuthRequired())
	r.GET("/test", func(c *gin.Context) {
		uid := c.GetInt64("user_id")
		uname := c.GetString("username")
		role := c.GetString("role")
		c.JSON(200, gin.H{"user_id": uid, "username": uname, "role": role})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthRequired_CookieToken(t *testing.T) {
	token, err := jwt.GenerateToken(2, "cookieuser", "user")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	r := gin.New()
	r.Use(AuthRequired())
	r.GET("/test", func(c *gin.Context) {
		uid := c.GetInt64("user_id")
		c.JSON(200, gin.H{"user_id": uid})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAdminRequired_AsUser(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		// Simulate AuthRequired having already set the role
		c.Set("role", "user")
		c.Next()
	})
	r.Use(AdminRequired())
	r.GET("/api/v1/admin/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestAdminRequired_AsUser_WebRedirect(t *testing.T) {
	// Web path with non-admin role redirects to /
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	r.Use(AdminRequired())
	r.GET("/admin/", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/" {
		t.Errorf("expected Location /, got %s", loc)
	}
}

func TestAdminRequired_AsAdmin(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.Use(AdminRequired())
	r.GET("/admin/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestOptionalAuth_NoToken(t *testing.T) {
	r := gin.New()
	r.Use(OptionalAuth())
	r.GET("/test", func(c *gin.Context) {
		uid := c.GetInt64("user_id")
		c.JSON(200, gin.H{"user_id": uid})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestOptionalAuth_WithToken(t *testing.T) {
	token, err := jwt.GenerateToken(3, "optuser", "user")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	r := gin.New()
	r.Use(OptionalAuth())
	r.GET("/test", func(c *gin.Context) {
		uid := c.GetInt64("user_id")
		c.JSON(200, gin.H{"user_id": uid})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRedirectIfAuth_LoggedIn(t *testing.T) {
	token, err := jwt.GenerateToken(4, "redirectuser", "user")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	r := gin.New()
	r.Use(RedirectIfAuth())
	r.GET("/login", func(c *gin.Context) {
		c.JSON(200, gin.H{"page": "login"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/login", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
}

func TestRedirectIfAuth_NotLoggedIn(t *testing.T) {
	r := gin.New()
	r.Use(RedirectIfAuth())
	r.GET("/login", func(c *gin.Context) {
		c.JSON(200, gin.H{"page": "login"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/login", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestJWT_RoundTrip(t *testing.T) {
	token, err := jwt.GenerateToken(100, "roundtrip", "admin")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	claims, err := jwt.ParseToken(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if claims.UserID != 100 {
		t.Errorf("UserID = %d, want 100", claims.UserID)
	}
	if claims.Username != "roundtrip" {
		t.Errorf("Username = %s, want roundtrip", claims.Username)
	}
	if claims.Role != "admin" {
		t.Errorf("Role = %s, want admin", claims.Role)
	}
}

func TestJWT_Expired(t *testing.T) {
	// Temporarily set short expiry
	oldDuration := time.Duration(72) * time.Hour
	jwt.Init("expiry-test-secret", -1) // force expired
	defer func() { jwt.Init("test-secret-for-middleware-tests", 72) }()
	_ = oldDuration

	token, err := jwt.GenerateToken(1, "expired", "user")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	_, err = jwt.ParseToken(token)
	if err == nil {
		t.Error("expected error for expired token, got nil")
	}
}
