package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/service"
)

type NotificationHandler struct {
	svc *service.NotificationService
}

func NewNotificationHandler() *NotificationHandler {
	return &NotificationHandler{svc: service.NewNotificationService()}
}

func (h *NotificationHandler) List(c *gin.Context) {
	userID := c.GetInt64("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	unreadOnly := c.Query("unread_only") == "true"

	list, total, err := h.svc.List(userID, unreadOnly, page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, response.PageResponse{
		List: list, Total: total, Page: page, PageSize: pageSize,
	})
}

func (h *NotificationHandler) MarkRead(c *gin.Context) {
	userID := c.GetInt64("user_id")
	notifID := getInt64Param(c, "id")
	if err := h.svc.MarkRead(notifID, userID); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	userID := c.GetInt64("user_id")
	if err := h.svc.MarkAllRead(userID); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *NotificationHandler) UnreadCount(c *gin.Context) {
	userID := c.GetInt64("user_id")
	count := h.svc.UnreadCount(userID)
	response.Success(c, gin.H{"count": count})
}

func (h *NotificationHandler) NotificationsPage(c *gin.Context) {
	c.HTML(http.StatusOK, "notifications.html", userData(c, gin.H{
		"title": "通知 - ShareO",
	}))
}
