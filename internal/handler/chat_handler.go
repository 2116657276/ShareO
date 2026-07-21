package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/zhoujianlin/ShareO/internal/middleware"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/service"
	"github.com/zhoujianlin/ShareO/internal/ws"
)

type ChatHandler struct {
	chatSvc       *service.ChatService
	hub           *ws.Hub
	trustedOrigin map[string]struct{}
}

func NewChatHandler(chatSvc *service.ChatService, hub *ws.Hub, trustedOrigins []string) *ChatHandler {
	origins := make(map[string]struct{}, len(trustedOrigins))
	for _, origin := range trustedOrigins {
		origin = strings.TrimRight(strings.TrimSpace(origin), "/")
		if origin != "" {
			origins[origin] = struct{}{}
		}
	}
	return &ChatHandler{chatSvc: chatSvc, hub: hub, trustedOrigin: origins}
}

func (h *ChatHandler) originAllowed(r *http.Request) bool {
	origin := strings.TrimRight(r.Header.Get("Origin"), "/")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	expectedScheme := "http"
	if r.TLS != nil {
		expectedScheme = "https"
	}
	if err == nil && strings.EqualFold(parsed.Scheme, expectedScheme) && strings.EqualFold(parsed.Host, r.Host) {
		return true
	}
	_, ok := h.trustedOrigin[origin]
	return ok
}

func (h *ChatHandler) ServeWS(c *gin.Context) {
	claims, _, err := middleware.AuthenticateRequest(c.Request, true)
	if err != nil {
		response.Unauthorized(c, "登录已失效，请重新登录")
		return
	}
	if err := h.chatSvc.ValidateUser(c.Request.Context(), claims.UserID); err != nil {
		response.Unauthorized(c, "账号不可用，请重新登录")
		return
	}
	upgrader := websocket.Upgrader{
		ReadBufferSize: 1024, WriteBufferSize: 1024, CheckOrigin: h.originAllowed,
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Warn("websocket upgrade failed", "user_id", claims.UserID, "err", err)
		return
	}
	pulse := func(userID int64) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := h.chatSvc.SetUserOnline(ctx, userID); err != nil {
			slog.Warn("failed to refresh websocket presence", "user_id", userID, "err", err)
		}
	}
	offline := func(userID int64) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := h.chatSvc.SetUserOffline(ctx, userID); err != nil {
			slog.Warn("failed to clear websocket presence", "user_id", userID, "err", err)
		}
	}
	client := ws.NewClient(h.hub, conn, claims.UserID, pulse, offline)
	client.Start()
	pulse(claims.UserID)
}

func (h *ChatHandler) ListConversations(c *gin.Context) {
	conversations, err := h.chatSvc.ListConversations(c.Request.Context(), c.GetInt64("user_id"))
	if err != nil {
		handleChatError(c, err)
		return
	}
	response.Success(c, conversations)
}

func (h *ChatHandler) CreateConversation(c *gin.Context) {
	userID := c.GetInt64("user_id")
	var req struct {
		UserID    int64   `json:"user_id"`
		Title     string  `json:"title"`
		MemberIDs []int64 `json:"member_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式无效")
		return
	}
	var conversation any
	var err error
	if req.UserID > 0 {
		conversation, err = h.chatSvc.EnsureDM(c.Request.Context(), userID, req.UserID)
	} else {
		conversation, err = h.chatSvc.CreateGroup(c.Request.Context(), userID, req.Title, req.MemberIDs)
	}
	if err != nil {
		handleChatError(c, err)
		return
	}
	response.Success(c, conversation)
}

func (h *ChatHandler) GetMessages(c *gin.Context) {
	convID := getInt64Param(c, "id")
	beforeID, ok := parsePositiveQuery(c, "before_id")
	if !ok {
		response.BadRequest(c, "before_id 必须为正整数")
		return
	}
	afterID, ok := parsePositiveQuery(c, "after_id")
	if !ok {
		response.BadRequest(c, "after_id 必须为正整数")
		return
	}
	limit := 30
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 100 {
			response.BadRequest(c, "limit 必须为 1-100 的整数")
			return
		}
		limit = parsed
	}
	messages, err := h.chatSvc.GetMessages(
		c.Request.Context(), convID, c.GetInt64("user_id"), beforeID, afterID, limit,
	)
	if err != nil {
		handleChatError(c, err)
		return
	}
	response.Success(c, gin.H{"messages": messages, "has_more": afterID == 0 && len(messages) >= limit})
}

func (h *ChatHandler) SendMessage(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "消息内容不能为空")
		return
	}
	message, err := h.chatSvc.SendMessage(
		c.Request.Context(), c.GetInt64("user_id"), getInt64Param(c, "id"), req.Content,
	)
	if err != nil {
		handleChatError(c, err)
		return
	}
	response.Success(c, message)
}

func (h *ChatHandler) MarkRead(c *gin.Context) {
	var req struct {
		MessageID int64 `json:"message_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "消息 ID 无效")
		return
	}
	if err := h.chatSvc.MarkRead(
		c.Request.Context(), getInt64Param(c, "id"), c.GetInt64("user_id"), req.MessageID,
	); err != nil {
		handleChatError(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *ChatHandler) UnreadCount(c *gin.Context) {
	count, err := h.chatSvc.UnreadCount(c.Request.Context(), c.GetInt64("user_id"))
	if err != nil {
		handleChatError(c, err)
		return
	}
	response.Success(c, gin.H{"total": count})
}

func (h *ChatHandler) InviteMembers(c *gin.Context) {
	var req struct {
		UserIDs []int64 `json:"user_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "至少选择一位成员")
		return
	}
	if err := h.chatSvc.InviteMembers(
		c.Request.Context(), getInt64Param(c, "id"), c.GetInt64("user_id"), req.UserIDs,
	); err != nil {
		handleChatError(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *ChatHandler) LeaveConversation(c *gin.Context) {
	if err := h.chatSvc.LeaveGroup(
		c.Request.Context(), getInt64Param(c, "id"), c.GetInt64("user_id"),
	); err != nil {
		handleChatError(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *ChatHandler) DissolveConversation(c *gin.Context) {
	if err := h.chatSvc.DissolveGroup(
		c.Request.Context(), getInt64Param(c, "id"), c.GetInt64("user_id"),
	); err != nil {
		handleChatError(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *ChatHandler) SearchUsers(c *gin.Context) {
	limit := 0
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			response.BadRequest(c, "limit 必须为正整数")
			return
		}
		limit = parsed
	}
	users, err := h.chatSvc.SearchUsers(
		c.Request.Context(), c.Query("q"), c.GetInt64("user_id"), limit,
	)
	if err != nil {
		handleChatError(c, err)
		return
	}
	response.Success(c, users)
}

func parsePositiveQuery(c *gin.Context, name string) (int64, bool) {
	raw := c.Query(name)
	if raw == "" {
		return 0, true
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	return value, err == nil && value > 0
}

func (h *ChatHandler) ChatPage(c *gin.Context) {
	c.HTML(http.StatusOK, "chat.html", userData(c, gin.H{"title": "消息 - ShareO"}))
}

func handleChatError(c *gin.Context, err error) {
	message := "操作失败"
	if _, detail, ok := strings.Cut(err.Error(), ": "); ok {
		message = detail
	}
	switch {
	case errors.Is(err, service.ErrChatInvalid):
		response.BadRequest(c, message)
	case errors.Is(err, service.ErrChatForbidden):
		response.Forbidden(c, message)
	case errors.Is(err, service.ErrChatNotFound):
		response.NotFound(c, message)
	case errors.Is(err, service.ErrChatConflict):
		response.Conflict(c, message)
	default:
		slog.Error("chat operation failed", "err", err)
		response.InternalError(c, "聊天服务暂时不可用")
	}
}
