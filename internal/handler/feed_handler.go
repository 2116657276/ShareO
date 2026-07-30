package handler

import (
	"errors"
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
		if errors.Is(err, service.ErrPostSearchUnavailable) {
			response.Error(c, http.StatusServiceUnavailable, response.ErrCodeInternal, "搜索暂不可用")
			return
		}
		response.InternalError(c, "搜索失败，请稍后重试")
		return
	}
	respondPage(c, page, pageSize, posts, total)
}

func (h *FeedHandler) GetFeed(c *gin.Context) {
	var req service.FeedReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	currentUserID := c.GetInt64("user_id")
	posts, total, err := h.svc.GetFeed(req, currentUserID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	totalPages := (int(total) + pageSize - 1) / pageSize

	response.Success(c, response.PageResponse{
		List:       posts,
		Total:      total,
		Page:       req.Page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

func (h *FeedHandler) GetFollowingFeed(c *gin.Context) {
	var req service.FeedReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
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
		List: posts, Total: total, Page: page, PageSize: pageSize,
		TotalPages: (int(total) + pageSize - 1) / pageSize,
	})
}

func (h *FeedHandler) HomePage(c *gin.Context) {
	currentUserID := c.GetInt64("user_id")
	sortBy := c.DefaultQuery("sort", "latest")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))

	req := service.FeedReq{
		Sort:     sortBy,
		Page:     page,
		PageSize: 12,
	}

	posts, total, err := h.svc.GetFeed(req, currentUserID)
	if err != nil {
		c.HTML(http.StatusOK, "feed.html", userData(c, gin.H{
			"title": "ShareO - 发现美好",
			"Error": err.Error(),
		}))
		return
	}

	totalPages := (int(total) + 11) / 12

	c.HTML(http.StatusOK, "feed.html", userData(c, gin.H{
		"title":      "ShareO - 发现美好",
		"Posts":      posts,
		"Total":      total,
		"Page":       page,
		"TotalPages": totalPages,
		"Sort":       sortBy,
	}))
}
