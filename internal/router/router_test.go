package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRemovedFeatureRoutesReturnNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldWorkingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Clean(filepath.Join(oldWorkingDir, "../.."))
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWorkingDir) })

	engine := SetupRouter()
	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/posts/1/favorite"},
		{http.MethodGet, "/api/v1/users/1/favorites"},
		{http.MethodPost, "/api/v1/posts/1/repost"},
		{http.MethodPost, "/api/v1/conversations/1/members"},
		{http.MethodDelete, "/api/v1/conversations/1/members/me"},
		{http.MethodDelete, "/api/v1/conversations/1"},
		{http.MethodGet, "/topic/1"},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, httptest.NewRequest(tt.method, tt.path, nil))
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", recorder.Code)
			}
		})
	}
}

func TestFavoriteRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := SetupRouter()
	for _, request := range []struct {
		method string
		path   string
	}{
		{http.MethodPut, "/api/v1/posts/1/favorite"},
		{http.MethodDelete, "/api/v1/posts/1/favorite"},
		{http.MethodGet, "/api/v1/favorites"},
	} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(request.method, request.path, nil))
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want %d", request.method, request.path, recorder.Code, http.StatusUnauthorized)
		}
	}
}

func TestFollowingPageRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := SetupRouter()
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/following", nil))
	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusFound)
	}
	if location := recorder.Header().Get("Location"); location != "/login" {
		t.Fatalf("location = %q, want /login", location)
	}
}

func TestFollowingFeedRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := SetupRouter()
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/feed/following?page=1&page_size=12", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}
