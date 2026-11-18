package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/pkg/jwt"
)

func allowTestSession(t *testing.T, token string) {
	t.Helper()
	previous := loginTokenLookup
	previousVersion := sessionVersionLookup
	loginTokenLookup = func(context.Context, int64) (string, error) {
		return token, nil
	}
	sessionVersionLookup = func(context.Context, int64) (int64, error) { return 1, nil }
	t.Cleanup(func() {
		loginTokenLookup = previous
		sessionVersionLookup = previousVersion
	})
}

func TestAuthenticateRequest_RedisFailureFailsClosed(t *testing.T) {
	token, err := jwt.GenerateToken(91, "redis-outage", "user")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	previous := loginTokenLookup
	loginTokenLookup = func(context.Context, int64) (string, error) {
		return "", errors.New("redis unavailable")
	}
	t.Cleanup(func() { loginTokenLookup = previous })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	_, _, err = AuthenticateRequest(req, false)
	if !errors.Is(err, ErrSessionUnavailable) {
		t.Fatalf("AuthenticateRequest error = %v, want ErrSessionUnavailable", err)
	}
}

func TestAuthRequired_SessionStoreUnavailableReturns503(t *testing.T) {
	token, err := jwt.GenerateToken(92, "redis-outage-api", "user")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	previous := loginTokenLookup
	loginTokenLookup = func(context.Context, int64) (string, error) {
		return "", errors.New("redis unavailable")
	}
	t.Cleanup(func() { loginTokenLookup = previous })

	r := gin.New()
	r.Use(AuthRequired())
	r.GET("/api/v1/auth/me", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestAuthenticateRequestRejectsRevokedSessionVersion(t *testing.T) {
	token, err := jwt.GenerateToken(93, "revoked-version", "user", 4)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	previousTokenLookup := loginTokenLookup
	previousVersionLookup := sessionVersionLookup
	loginTokenLookup = func(context.Context, int64) (string, error) { return token, nil }
	sessionVersionLookup = func(context.Context, int64) (int64, error) { return 5, nil }
	t.Cleanup(func() {
		loginTokenLookup = previousTokenLookup
		sessionVersionLookup = previousVersionLookup
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	_, _, err = AuthenticateRequest(req, false)
	if !errors.Is(err, ErrReplacedToken) {
		t.Fatalf("AuthenticateRequest error = %v, want ErrReplacedToken", err)
	}
}
