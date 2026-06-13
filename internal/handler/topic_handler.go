package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/service"
)

type TopicHandler struct {
	svc *service.TopicService
}

func NewTopicHandler() *TopicHandler {
	return &TopicHandler{svc: service.NewTopicService()}
}

func (h *TopicHandler) TopicPage(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	data, err := h.svc.GetTopicPage(c.Param("id"), page, pageSize)
	if err != nil || data == nil {
		c.HTML(http.StatusNotFound, "404.html", userData(c, gin.H{"title": "话题不存在"}))
		return
	}

	c.HTML(http.StatusOK, "topic.html", userData(c, gin.H{
		"title":      "#" + data.Topic.Name + " - ShareO",
		"Topic":      data.Topic,
		"Posts":      data.Posts,
		"TotalPosts": data.TotalPosts,
	}))
}
