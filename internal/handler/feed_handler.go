package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/service"
)

type FeedHandler struct {
	svc *service.FeedService
}

func NewFeedHandler() *FeedHandler { return &FeedHandler{svc: service.NewFeedService()} }

func normalizeFeedSort(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "hot":
		return "hot"
	default:
		return "latest"
	}
}

func (h *FeedHandler) Search(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" || utf8.RuneCountInString(q) > 200 {
		response.BadRequest(c, "搜索内容长度必须为 1-200 个字符")
		return
	}
	page, pageSize := getPageSizePair(c, 12)
	currentUserID := c.GetInt64("user_id")

	posts, total, err := h.svc.Search(c.Request.Context(), q, page, pageSize, currentUserID)
	if err != nil {
		if errors.Is(err, service.ErrAISearchBusy) {
			response.Error(c, http.StatusTooManyRequests, response.ErrCodeRateLimit, "搜索请求较多，请稍后再试")
			return
		}
		if errors.Is(err, service.ErrPostSearchUnavailable) {
			response.Error(c, http.StatusServiceUnavailable, response.ErrCodeInternal, "搜索暂不可用")
			return
		}
		response.InternalError(c, "搜索失败，请稍后重试")
		return
	}
	respondPage(c, page, pageSize, service.ToPublicPosts(posts), total)
}

func (h *FeedHandler) GetFeed(c *gin.Context) {
	var req service.FeedReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "分页参数格式错误")
		return
	}
	currentUserID := c.GetInt64("user_id")
	posts, total, err := h.svc.GetFeed(req, currentUserID)
	if err != nil {
		slog.Warn("feed lookup failed", "user_id", currentUserID, "err", err)
		response.InternalError(c, "动态暂时无法加载，请稍后重试")
		return
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	totalPages := (int(total) + pageSize - 1) / pageSize

	response.Success(c, response.PageResponse{
		List:       service.ToPublicPosts(posts),
		Total:      total,
		Page:       req.Page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

func (h *FeedHandler) GetFollowingFeed(c *gin.Context) {
	var req service.FeedReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "分页参数格式错误")
		return
	}
	posts, total, err := h.svc.GetFollowingFeed(req, c.GetInt64("user_id"))
	if err != nil {
		response.InternalError(c, "关注动态加载失败，请稍后重试")
		return
	}
	pageSize := req.PageSize
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	response.Success(c, response.PageResponse{
		List: service.ToPublicPosts(posts), Total: total, Page: page, PageSize: pageSize,
		TotalPages: (int(total) + pageSize - 1) / pageSize,
	})
}

func (h *FeedHandler) HomePage(c *gin.Context) {
	currentUserID := c.GetInt64("user_id")
	sortBy := normalizeFeedSort(c.DefaultQuery("sort", "latest"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page <= 0 {
		page = 1
	}

	req := service.FeedReq{
		Sort:     sortBy,
		Page:     page,
		PageSize: 12,
	}

	posts, total, err := h.svc.GetFeed(req, currentUserID)
	if err != nil {
		slog.Warn("home feed lookup failed", "user_id", currentUserID, "err", err)
		c.HTML(http.StatusOK, "feed.html", userData(c, gin.H{
			"title": "ShareO - 发现美好",
			"Error": "动态暂时无法加载，请稍后重试",
		}))
		return
	}

	totalPages := (int(total) + 11) / 12

	c.HTML(http.StatusOK, "feed.html", userData(c, gin.H{
		"title":      "ShareO - 发现美好",
		"Posts":      service.ToPublicPosts(posts),
		"Total":      total,
		"Page":       page,
		"TotalPages": totalPages,
		"Sort":       sortBy,
	}))
}
