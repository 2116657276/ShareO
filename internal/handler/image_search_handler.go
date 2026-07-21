package handler

import (
	"encoding/json"
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

type ImageSearchHandler struct {
	postRepo  *repository.PostRepo
	client    *http.Client
	aiBaseURL string
	token     string
}

func NewImageSearchHandler(postRepo *repository.PostRepo) *ImageSearchHandler {
	if postRepo == nil {
		postRepo = repository.NewPostRepo()
	}
	baseURL := strings.TrimRight(os.Getenv("SHAREO_AI_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8000"
	}
	return &ImageSearchHandler{
		postRepo:  postRepo,
		client:    &http.Client{Timeout: 5 * time.Second},
		aiBaseURL: baseURL,
		token:     os.Getenv("SHAREO_INTERNAL_TOKEN"),
	}
}

func (h *ImageSearchHandler) Search(c *gin.Context) {
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
		response.Error(c, http.StatusServiceUnavailable, response.ErrCodeInternal, "语义搜图暂不可用")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
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
	results := make([]gin.H, 0, len(ai.Results))
	for _, item := range ai.Results {
		post, ok := postByID[item.PostID]
		if !ok {
			continue
		}
		results = append(results, gin.H{
			"image_id": item.ImageID, "post_id": item.PostID,
			"object_key": item.ObjectKey, "score": item.Score,
			"post": post,
		})
	}
	response.Success(c, gin.H{"query": query, "results": results})
}
