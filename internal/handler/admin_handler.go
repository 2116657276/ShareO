package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/service"
)

type AdminHandler struct {
	svc *service.AdminService
}

func NewAdminHandler() *AdminHandler { return &AdminHandler{svc: service.NewAdminService()} }

// --- Dashboard ---

func (h *AdminHandler) Dashboard(c *gin.Context) {
	stats, err := h.svc.GetDashboardStats()
	if err != nil {
		c.HTML(http.StatusOK, "admin_dashboard.html", userData(c, gin.H{"Error": err.Error()}))
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
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, stats)
}

// --- Post Review ---

func (h *AdminHandler) Review(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	posts, total, err := h.svc.GetPendingPosts(page, 20)
	if err != nil {
		c.HTML(http.StatusOK, "admin_review.html", userData(c, gin.H{"Error": err.Error()}))
		return
	}
	totalPages := (int(total) + 19) / 20
	c.HTML(http.StatusOK, "admin_review.html", userData(c, gin.H{
		"title":      "审核管理 - ShareO",
		"Posts":      posts,
		"Total":      total,
		"Page":       page,
		"TotalPages": totalPages,
})) 
}

func (h *AdminHandler) DeletePost(c *gin.Context) {
	postID := getInt64Param(c, "id")
	if err := h.svc.DeletePost(postID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *AdminHandler) GetPendingPosts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	posts, total, err := h.svc.GetPendingPosts(page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, response.PageResponse{
		List: posts, Total: total, Page: page, PageSize: pageSize,
	})
}

func (h *AdminHandler) GetUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	role := c.DefaultQuery("role", "")
	users, total, err := h.svc.ListUsers(page, pageSize, role, nil)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, response.PageResponse{
		List: users, Total: total, Page: page, PageSize: pageSize,
	})
}

func (h *AdminHandler) GetLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	action := c.DefaultQuery("action", "")
	logs, total, err := h.svc.GetLogs(page, pageSize, nil, action)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, response.PageResponse{
		List: logs, Total: total, Page: page, PageSize: pageSize,
	})
}

func (h *AdminHandler) ApprovePost(c *gin.Context) {
	reviewerID := c.GetInt64("user_id")
	postID := getInt64Param(c, "id")
	if err := h.svc.ReviewPost(postID, "approved", "", reviewerID); err != nil {
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
	c.ShouldBindJSON(&req)
	if err := h.svc.ReviewPost(postID, "rejected", req.Comment, reviewerID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// --- User Management ---

func (h *AdminHandler) UsersPage(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	role := c.DefaultQuery("role", "")
	users, total, err := h.svc.ListUsers(page, 20, role, nil)
	if err != nil {
		c.HTML(http.StatusOK, "admin_users.html", userData(c, gin.H{"Error": err.Error()}))
		return
	}
	totalPages := (int(total) + 19) / 20
	c.HTML(http.StatusOK, "admin_users.html", userData(c, gin.H{
		"title":      "用户管理 - ShareO",
		"Users":      users,
		"Total":      total,
		"Page":       page,
		"TotalPages": totalPages,
})) 
}

func (h *AdminHandler) UpdateUserStatus(c *gin.Context) {
	userID := getInt64Param(c, "id")
	var req struct {
		Status int8 `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.svc.UpdateUserStatus(userID, req.Status); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// --- Logs ---

func (h *AdminHandler) LogsPage(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	action := c.DefaultQuery("action", "")
	logs, total, err := h.svc.GetLogs(page, 20, nil, action)
	if err != nil {
		c.HTML(http.StatusOK, "admin_logs.html", userData(c, gin.H{"Error": err.Error()}))
		return
	}
	totalPages := (int(total) + 19) / 20
	c.HTML(http.StatusOK, "admin_logs.html", userData(c, gin.H{
		"title":      "系统日志 - ShareO",
		"Logs":       logs,
		"Total":      total,
		"Page":       page,
		"TotalPages": totalPages,
})) 
}
