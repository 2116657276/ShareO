package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/zhoujianlin/ShareO/internal/pkg/jwt"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/service"
	"github.com/zhoujianlin/ShareO/internal/ws"
)

// ChatHandler handles IM REST endpoints and WebSocket upgrade.
type ChatHandler struct {
	chatSvc *service.ChatService
	hub     *ws.Hub
}

// NewChatHandler creates a ChatHandler with injected dependencies.
func NewChatHandler(chatSvc *service.ChatService, hub *ws.Hub) *ChatHandler {
	return &ChatHandler{chatSvc: chatSvc, hub: hub}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // same-origin requests (browser pages) have no Origin header
		}
		// Allow localhost origins for development
		if strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "http://127.0.0.1:") {
			return true
		}
		// In production, add the deployed domain to the allowed origins
		return false
	},
}

// ServeWS handles WebSocket upgrade requests.
// Authentication is done via Cookie (same as web pages) or query param for native clients.
func (h *ChatHandler) ServeWS(c *gin.Context) {
	// Authenticate via Cookie token (same as AuthRequired middleware)
	var userID int64
	token, err := c.Cookie("token")
	if err == nil && token != "" {
		claims, parseErr := jwt.ParseToken(token)
		if parseErr == nil {
			userID = claims.UserID
		}
	}
	// Fallback: Authorization header for native clients (e.g., mobile apps)
	if userID == 0 {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			claims, parseErr := jwt.ParseToken(strings.TrimPrefix(authHeader, "Bearer "))
			if parseErr == nil {
				userID = claims.UserID
			}
		}
	}
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Warn("ws upgrade failed", "user_id", userID, "err", err)
		return
	}

	client := ws.NewClient(h.hub, conn, userID)
	client.Start()

	// Mark user online (best-effort; heartbeat keeps it alive)
	if err := h.chatSvc.SetUserOnline(context.Background(), userID); err != nil {
		slog.Warn("failed to set user online", "user_id", userID, "err", err)
	}
}

// --- REST: Conversations ---

// ListConversations GET /api/v1/conversations
func (h *ChatHandler) ListConversations(c *gin.Context) {
	userID := c.GetInt64("user_id")
	convs, err := h.chatSvc.ListConversations(context.Background(), userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, convs)
}

// CreateConversation POST /api/v1/conversations
func (h *ChatHandler) CreateConversation(c *gin.Context) {
	userID := c.GetInt64("user_id")
	var req struct {
		UserID    int64   `json:"user_id"`    // for DM
		Title     string  `json:"title"`      // for group
		MemberIDs []int64 `json:"member_ids"` // for group
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	ctx := context.Background()
	if req.UserID > 0 {
		// DM
		conv, err := h.chatSvc.EnsureDM(ctx, userID, req.UserID)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Success(c, conv)
	} else {
		// Group
		conv, err := h.chatSvc.CreateGroup(ctx, userID, req.Title, req.MemberIDs)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Success(c, conv)
	}
}

// --- REST: Messages ---

// GetMessages GET /api/v1/conversations/:id/messages
func (h *ChatHandler) GetMessages(c *gin.Context) {
	userID := c.GetInt64("user_id")
	convID := getInt64Param(c, "id")
	beforeID, _ := strconv.ParseInt(c.DefaultQuery("before_id", "0"), 10, 64)
	limit := 30
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	msgs, err := h.chatSvc.GetMessages(context.Background(), convID, userID, beforeID, limit)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	hasMore := len(msgs) >= limit
	response.Success(c, gin.H{
		"messages": msgs,
		"has_more": hasMore,
	})
}

// SendMessage POST /api/v1/conversations/:id/messages
func (h *ChatHandler) SendMessage(c *gin.Context) {
	userID := c.GetInt64("user_id")
	convID := getInt64Param(c, "id")
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	msg, err := h.chatSvc.SendMessage(context.Background(), userID, convID, req.Content)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, msg)
}

// MarkRead PUT /api/v1/conversations/:id/read
func (h *ChatHandler) MarkRead(c *gin.Context) {
	userID := c.GetInt64("user_id")
	convID := getInt64Param(c, "id")
	var req struct {
		MessageID int64 `json:"message_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.chatSvc.MarkRead(context.Background(), convID, userID, req.MessageID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// UnreadCount GET /api/v1/conversations/unread-count
func (h *ChatHandler) UnreadCount(c *gin.Context) {
	userID := c.GetInt64("user_id")
	count, err := h.chatSvc.UnreadCount(context.Background(), userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"total": count})
}

// JoinConversation POST /api/v1/conversations/:id/join
func (h *ChatHandler) JoinConversation(c *gin.Context) {
	userID := c.GetInt64("user_id")
	convID := getInt64Param(c, "id")
	if err := h.chatSvc.JoinGroup(context.Background(), convID, userID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// LeaveConversation POST /api/v1/conversations/:id/leave
// ChatPage renders the chat web page.
func (h *ChatHandler) ChatPage(c *gin.Context) {
	c.HTML(http.StatusOK, "chat.html", userData(c, gin.H{
		"title": "消息 - ShareO",
	}))
}

func (h *ChatHandler) LeaveConversation(c *gin.Context) {
	userID := c.GetInt64("user_id")
	convID := getInt64Param(c, "id")
	if err := h.chatSvc.LeaveGroup(context.Background(), convID, userID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}
