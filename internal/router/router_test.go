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
		{http.MethodGet, "/api/v1/favorites"},
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
