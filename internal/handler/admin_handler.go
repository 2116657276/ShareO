package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/repository"
	"github.com/zhoujianlin/ShareO/internal/service"
)

type AdminHandler struct {
	svc          *service.AdminService
	disconnector interface{ DisconnectUser(int64) }
}

func NewAdminHandler(disconnector ...interface{ DisconnectUser(int64) }) *AdminHandler {
	h := &AdminHandler{svc: service.NewAdminService()}
	if len(disconnector) > 0 {
		h.disconnector = disconnector[0]
	}
	return h
}

// --- Dashboard ---

func (h *AdminHandler) Dashboard(c *gin.Context) {
	stats, err := h.svc.GetDashboardStats()
	if err != nil {
		slog.Warn("admin dashboard lookup failed", "err", err)
		c.HTML(http.StatusOK, "admin_dashboard.html", userData(c, gin.H{"Error": "后台数据暂时无法加载，请稍后重试"}))
		return
	}
	c.HTML(http.StatusOK, "admin_dashboard.html", userData(c, gin.H{
		"title": "管理后台 - ShareO",
		"Stats": stats,
	}))
}

func (h *AdminHandler) GetStats(c *gin.Context) {
	stats, err := h.svc.GetDashboardStats()
	if err != nil {
		slog.Warn("admin stats lookup failed", "err", err)
		response.InternalError(c, "后台数据暂时无法加载，请稍后重试")
		return
	}
	response.Success(c, stats)
}

// --- Post Review ---

func (h *AdminHandler) Review(c *gin.Context) {
	page := getPage(c)
	posts, total, err := h.svc.GetPendingPosts(page, 20)
	if err != nil {
		slog.Warn("admin review lookup failed", "err", err)
		c.HTML(http.StatusOK, "admin_review.html", userData(c, gin.H{"Error": "审核列表暂时无法加载，请稍后重试"}))
		return
	}
	c.HTML(http.StatusOK, "admin_review.html", userData(c, gin.H{
		"title":      "审核管理 - ShareO",
		"Posts":      posts,
		"Total":      total,
		"Page":       page,
		"TotalPages": calcPages(total, 20),
	}))
}

func (h *AdminHandler) DeletePost(c *gin.Context) {
	postID := getInt64Param(c, "id")
	adminID := c.GetInt64("user_id")
	if err := h.svc.DeletePost(postID, adminID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *AdminHandler) GetPendingPosts(c *gin.Context) {
	page, pageSize := getPageSizePair(c, 50)
	posts, total, err := h.svc.GetPendingPosts(page, pageSize)
	if err != nil {
		slog.Warn("admin pending posts lookup failed", "err", err)
		response.InternalError(c, "审核列表暂时无法加载，请稍后重试")
		return
	}
	respondPage(c, page, pageSize, posts, total)
}

func (h *AdminHandler) GetUsers(c *gin.Context) {
	page, pageSize := getPageSizePair(c, 50)
	role := c.DefaultQuery("role", "")
	users, total, err := h.svc.ListUsers(page, pageSize, role, nil)
	if err != nil {
		slog.Warn("admin users lookup failed", "err", err)
		response.InternalError(c, "用户列表暂时无法加载，请稍后重试")
		return
	}
	respondPage(c, page, pageSize, users, total)
}

func (h *AdminHandler) GetLogs(c *gin.Context) {
	page, pageSize := getPageSizePair(c, 50)
	action := c.DefaultQuery("action", "")
	logs, total, err := h.svc.GetLogs(page, pageSize, nil, action)
	if err != nil {
		slog.Warn("admin logs lookup failed", "err", err)
		response.InternalError(c, "系统日志暂时无法加载，请稍后重试")
		return
	}
	respondPage(c, page, pageSize, logs, total)
}

func (h *AdminHandler) ApprovePost(c *gin.Context) {
	reviewerID := c.GetInt64("user_id")
	postID := getInt64Param(c, "id")
	// Approve without overwriting review_comment (preserve any previous rejection reason)
	if err := h.svc.ReviewPost(postID, model.StatusApproved, "", reviewerID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *AdminHandler) RejectPost(c *gin.Context) {
	reviewerID := c.GetInt64("user_id")
	postID := getInt64Param(c, "id")
	var req struct {
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数格式错误")
		return
	}
	if err := h.svc.ReviewPost(postID, model.StatusRejected, req.Comment, reviewerID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// --- User Management ---

func (h *AdminHandler) UsersPage(c *gin.Context) {
	page := getPage(c)
	role := c.DefaultQuery("role", "")
	users, total, err := h.svc.ListUsers(page, 20, role, nil)
	if err != nil {
		slog.Warn("admin users page lookup failed", "err", err)
		c.HTML(http.StatusOK, "admin_users.html", userData(c, gin.H{"Error": "用户列表暂时无法加载，请稍后重试"}))
		return
	}
	c.HTML(http.StatusOK, "admin_users.html", userData(c, gin.H{
		"title":      "用户管理 - ShareO",
		"Users":      users,
		"Total":      total,
		"Page":       page,
		"TotalPages": calcPages(total, 20),
	}))
}

func (h *AdminHandler) UpdateUserStatus(c *gin.Context) {
	adminID := c.GetInt64("user_id")
	userID := getInt64Param(c, "id")
	var req struct {
		Status int8 `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.svc.UpdateUserStatus(adminID, userID, req.Status); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.Status == model.UserStatusBanned && h.disconnector != nil {
		if err := repository.DeleteLoginToken(c.Request.Context(), userID); err != nil {
			slog.Warn("failed to revoke banned user's login cache", "user_id", userID, "err", err)
		}
		h.disconnector.DisconnectUser(userID)
	}
	response.Success(c, nil)
}

// --- Logs ---

func (h *AdminHandler) LogsPage(c *gin.Context) {
	page := getPage(c)
	action := c.DefaultQuery("action", "")
	logs, total, err := h.svc.GetLogs(page, 20, nil, action)
	if err != nil {
		slog.Warn("admin logs page lookup failed", "err", err)
		c.HTML(http.StatusOK, "admin_logs.html", userData(c, gin.H{"Error": "系统日志暂时无法加载，请稍后重试"}))
		return
	}
	c.HTML(http.StatusOK, "admin_logs.html", userData(c, gin.H{
		"title":      "系统日志 - ShareO",
		"Logs":       logs,
		"Total":      total,
		"Page":       page,
		"TotalPages": calcPages(total, 20),
	}))
}
