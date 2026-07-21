package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

// InternalHandler handles /internal/* routes for ai-service communication.
type InternalHandler struct {
	postRepo *repository.PostRepo
}

func NewInternalHandler(postRepo *repository.PostRepo) *InternalHandler {
	if postRepo == nil {
		postRepo = repository.NewPostRepo()
	}
	return &InternalHandler{postRepo: postRepo}
}

// HealthCheck returns service health for internal monitoring by ai-service.
func (h *InternalHandler) HealthCheck(c *gin.Context) {
	response.Success(c, gin.H{"service": "shareo", "status": "ok"})
}

func (h *InternalHandler) IndexPayload(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "无效的帖子 ID")
		return
	}
	payload, err := h.postRepo.GetIndexPayload(id)
	if err != nil {
		response.InternalError(c, "索引数据暂不可用")
		return
	}
	if payload == nil {
		c.Status(http.StatusNotFound)
		return
	}
	response.Success(c, payload)
}
