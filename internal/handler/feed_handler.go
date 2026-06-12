package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/service"
)

type FeedHandler struct {
	svc *service.FeedService
}

func NewFeedHandler() *FeedHandler { return &FeedHandler{svc: service.NewFeedService()} }

func (h *FeedHandler) Search(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		response.BadRequest(c, "搜索关键词不能为空")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "12"))
	currentUserID := c.GetInt64("user_id")

	posts, total, err := h.svc.Search(q, page, pageSize, currentUserID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	totalPages := (int(total) + pageSize - 1) / pageSize

	response.Success(c, response.PageResponse{
		List:       posts,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
})
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

func (h *FeedHandler) HomePage(c *gin.Context) {
	currentUserID := c.GetInt64("user_id")
	sortBy := c.DefaultQuery("sort", "latest")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))

	var topicID *int64
	if tidStr := c.Query("topic_id"); tidStr != "" {
		tid, _ := strconv.ParseInt(tidStr, 10, 64)
		topicID = &tid
	}

	req := service.FeedReq{
		Sort:     sortBy,
		TopicID:  topicID,
		Page:     page,
		PageSize: 12,
	}

	posts, total, err := h.svc.GetFeed(req, currentUserID)
	if err != nil {
		c.HTML(http.StatusOK, "feed.html", gin.H{
			"title": "ShareO - 发现美好",
			"Error": err.Error(),
})
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
