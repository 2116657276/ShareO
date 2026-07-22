package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/repository"
	"github.com/zhoujianlin/ShareO/internal/service"
)

// InternalHandler handles /internal/* routes for ai-service communication.
type InternalHandler struct {
	postRepo *repository.PostRepo
	chatSvc  *service.ChatService
}

func NewInternalHandler(postRepo *repository.PostRepo, chatServices ...*service.ChatService) *InternalHandler {
	if postRepo == nil {
		postRepo = repository.NewPostRepo()
	}
	handler := &InternalHandler{postRepo: postRepo}
	if len(chatServices) > 0 {
		handler.chatSvc = chatServices[0]
	}
	return handler
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

func (h *InternalHandler) ListIndexPayloads(c *gin.Context) {
	afterID, err := strconv.ParseInt(c.DefaultQuery("after_id", "0"), 10, 64)
	if err != nil || afterID < 0 {
		response.BadRequest(c, "after_id 必须是非负整数")
		return
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if err != nil || limit < 1 || limit > 200 {
		response.BadRequest(c, "limit 必须为 1-200")
		return
	}
	payloads, err := h.postRepo.ListIndexPayloads(afterID, limit)
	if err != nil {
		response.InternalError(c, "索引清单暂不可用")
		return
	}
	nextAfterID := afterID
	if len(payloads) > 0 {
		nextAfterID = payloads[len(payloads)-1].PostID
	}
	response.Success(c, gin.H{"items": payloads, "next_after_id": nextAfterID})
}

func (h *InternalHandler) BotTask(c *gin.Context) {
	if h.chatSvc == nil {
		response.InternalError(c, "Bot 服务未配置")
		return
	}
	messageID, err := strconv.ParseInt(c.Param("message_id"), 10, 64)
	if err != nil || messageID <= 0 {
		response.BadRequest(c, "消息 ID 无效")
		return
	}
	task, err := h.chatSvc.GetBotTask(c.Request.Context(), messageID)
	if err != nil {
		handleChatError(c, err)
		return
	}
	response.Success(c, task)
}

func (h *InternalHandler) BotReply(c *gin.Context) {
	if h.chatSvc == nil {
		response.InternalError(c, "Bot 服务未配置")
		return
	}
	var req struct {
		SourceMessageID int64               `json:"source_message_id" binding:"required"`
		ConversationID  int64               `json:"conversation_id" binding:"required"`
		Content         string              `json:"content" binding:"required"`
		Citations       []model.BotCitation `json:"citations"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Bot 回复格式无效")
		return
	}
	message, err := h.chatSvc.ReplyAsBot(
		c.Request.Context(), req.SourceMessageID, req.ConversationID, req.Content, req.Citations,
	)
	if err != nil {
		handleChatError(c, err)
		return
	}
	response.Success(c, message)
}
