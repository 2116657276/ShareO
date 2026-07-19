package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
)

// InternalHandler handles /internal/* routes for ai-service communication.
type InternalHandler struct{}

func NewInternalHandler() *InternalHandler {
	return &InternalHandler{}
}

// HealthCheck returns service health for internal monitoring by ai-service.
func (h *InternalHandler) HealthCheck(c *gin.Context) {
	response.Success(c, gin.H{"service": "shareo", "status": "ok"})
}
