package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

type imageSearchAIResult struct {
	ImageID   int64   `json:"image_id"`
	PostID    int64   `json:"post_id"`
	ObjectKey string  `json:"object_key"`
	CreatedAt string  `json:"created_at"`
	Score     float64 `json:"score"`
}

type imageSearchAIResponse struct {
	Results []imageSearchAIResult `json:"results"`
}

type imageSearchPostReader interface {
	FindByIDs([]int64) ([]model.Post, error)
}

type ImageSearchHandler struct {
	postRepo  imageSearchPostReader
	client    *http.Client
	aiBaseURL string
	token     string
}

// ImageSearchPage renders the authenticated shell for semantic image search.
// The browser calls the existing public API; no AI credentials cross this boundary.
func (h *ImageSearchHandler) ImageSearchPage(c *gin.Context) {
	c.HTML(http.StatusOK, "image_search.html", userData(c, gin.H{
		"title": "语义搜图 - ShareO",
	}))
}

func NewImageSearchHandler(postRepo imageSearchPostReader) *ImageSearchHandler {
	if postRepo == nil {
		postRepo = repository.NewPostRepo()
	}
	baseURL := strings.TrimRight(os.Getenv("SHAREO_AI_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8000"
	}
	return &ImageSearchHandler{
		postRepo:  postRepo,
		client:    &http.Client{Timeout: 8 * time.Second},
		aiBaseURL: baseURL,
		token:     os.Getenv("SHAREO_INTERNAL_TOKEN"),
	}
}

func (h *ImageSearchHandler) Search(c *gin.Context) {
	started := time.Now()
	query := strings.TrimSpace(c.Query("q"))
	if query == "" || utf8.RuneCountInString(query) > 200 {
		response.BadRequest(c, "搜索描述长度必须为 1-200 个字符")
		return
	}
	limit := 10
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 20 {
			response.BadRequest(c, "limit 必须为 1-20")
			return
		}
		limit = parsed
	}

	body, _ := json.Marshal(map[string]any{"query": query, "limit": limit})
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, h.aiBaseURL+"/v1/search/images", strings.NewReader(string(body)))
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, response.ErrCodeInternal, "语义搜图暂不可用")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if h.token != "" {
		req.Header.Set("X-Internal-Token", h.token)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		slog.Warn("semantic image search request failed", "duration_ms", time.Since(started).Milliseconds(), "err", err)
		response.Error(c, http.StatusServiceUnavailable, response.ErrCodeInternal, "语义搜图暂不可用")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Warn("semantic image search unavailable", "status", resp.StatusCode, "duration_ms", time.Since(started).Milliseconds())
		response.Error(c, http.StatusServiceUnavailable, response.ErrCodeInternal, "语义搜图暂不可用")
		return
	}
	var ai imageSearchAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&ai); err != nil {
		response.Error(c, http.StatusServiceUnavailable, response.ErrCodeInternal, "语义搜图响应无效")
		return
	}

	ids := make([]int64, 0, len(ai.Results))
	seen := make(map[int64]struct{}, len(ai.Results))
	for _, item := range ai.Results {
		if item.PostID > 0 {
			if _, ok := seen[item.PostID]; !ok {
				seen[item.PostID] = struct{}{}
				ids = append(ids, item.PostID)
			}
		}
	}
	posts, err := h.postRepo.FindByIDs(ids)
	if err != nil {
		response.InternalError(c, "搜索结果暂不可用")
		return
	}
	postByID := make(map[int64]model.Post, len(posts))
	for _, post := range posts {
		postByID[post.ID] = post
	}
	results := make([]gin.H, 0, limit)
	returnedPosts := make(map[int64]struct{}, limit)
	for _, item := range ai.Results {
		if len(results) >= limit {
			break
		}
		if _, duplicate := returnedPosts[item.PostID]; duplicate {
			continue
		}
		post, ok := postByID[item.PostID]
		if !ok {
			continue
		}
		returnedPosts[item.PostID] = struct{}{}
		results = append(results, gin.H{
			"image_id": item.ImageID, "post_id": item.PostID,
			"image_url": "/api/v1/images/" + strings.TrimPrefix(item.ObjectKey, "/"),
			"score":     item.Score,
			"post":      post,
		})
	}
	slog.Info("semantic image search complete", "query_length", utf8.RuneCountInString(query), "results", len(results), "duration_ms", time.Since(started).Milliseconds())
	response.Success(c, gin.H{"query": query, "results": results})
}
