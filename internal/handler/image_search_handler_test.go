package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/model"
)

type fakeImageSearchPosts struct {
	posts []model.Post
}

func TestImageSearchPageContract(t *testing.T) {
	content, err := os.ReadFile("../../web/templates/search/image_search.html")
	if err != nil {
		t.Fatal(err)
	}
	page := string(content)
	for _, required := range []string{
		"/api/v1/search",
		"/api/v1/search/images",
		"1–200",
		"searchResults",
		"mode",
		"imageSearchEmpty",
		"imageSearchUnavailable",
		"/post/${postID}",
	} {
		if !strings.Contains(page, required) {
			t.Fatalf("image search page missing %q", required)
		}
	}
}

func (f *fakeImageSearchPosts) FindByIDs(_ []int64) ([]model.Post, error) {
	return f.posts, nil
}

func TestImageSearchDeduplicatesPostsAndHidesObjectKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"image_id":11,"post_id":1,"object_key":"posts/medium/a.jpg","score":0.9},{"image_id":12,"post_id":1,"object_key":"posts/medium/b.jpg","score":0.8},{"image_id":21,"post_id":2,"object_key":"posts/medium/c.jpg","score":0.7}]}`))
	}))
	defer ai.Close()

	h := NewImageSearchHandler(&fakeImageSearchPosts{posts: []model.Post{{ID: 1}, {ID: 2}}})
	h.aiBaseURL = ai.URL
	h.client = ai.Client()
	router := gin.New()
	router.GET("/api/v1/search/images", h.Search)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/search/images?q=山&limit=2", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data struct {
			Results []struct {
				PostID   int64  `json:"post_id"`
				ImageID  int64  `json:"image_id"`
				ImageURL string `json:"image_url"`
			} `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data.Results) != 2 || response.Data.Results[0].ImageID != 11 {
		t.Fatalf("unexpected results: %+v", response.Data.Results)
	}
	if response.Data.Results[0].ImageURL != "/api/v1/images/posts/medium/a.jpg" {
		t.Fatalf("unexpected image url: %s", response.Data.Results[0].ImageURL)
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte("object_key")) {
		t.Fatal("public response must not expose object_key")
	}
}

func TestImageSearchMapsAITimeoutToServiceUnavailable(t *testing.T) {
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ai.Close()
	h := NewImageSearchHandler(&fakeImageSearchPosts{})
	h.aiBaseURL = ai.URL
	h.client = &http.Client{Timeout: 5 * time.Millisecond}
	router := gin.New()
	router.GET("/api/v1/search/images", h.Search)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/search/images?q=山", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
