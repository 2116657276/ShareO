package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/service"
)

type PostHandler struct {
	svc *service.PostService
}

func NewPostHandler() *PostHandler { return &PostHandler{svc: service.NewPostService()} }

func (h *PostHandler) Create(c *gin.Context) {
	userID := c.GetInt64("user_id")
	var req service.CreatePostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	post, err := h.svc.Create(userID, req)
	if err != nil {
		if errors.Is(err, service.ErrPostInvalid) {
			response.BadRequest(c, "帖子内容或图片参数无效")
		} else {
			slog.Warn("post creation failed", "user_id", userID, "err", err)
			response.InternalError(c, "帖子暂时无法发布，请稍后重试")
		}
		return
	}
	response.Success(c, post)
}

func (h *PostHandler) Update(c *gin.Context) {
	userID := c.GetInt64("user_id")
	postID := getInt64Param(c, "id")
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	post, err := h.svc.Update(userID, postID, req.Content)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPostInvalid):
			response.BadRequest(c, "帖子内容或图片参数无效")
		case errors.Is(err, service.ErrPostNotFound):
			response.NotFound(c, "帖子不存在")
		case errors.Is(err, service.ErrPermissionDenied):
			response.Forbidden(c, "无权编辑此帖子")
		default:
			slog.Warn("post update failed", "user_id", userID, "post_id", postID, "err", err)
			response.InternalError(c, "帖子暂时无法更新，请稍后重试")
		}
		return
	}
	response.Success(c, post)
}

func (h *PostHandler) Delete(c *gin.Context) {
	userID := c.GetInt64("user_id")
	postID := getInt64Param(c, "id")
	if err := h.svc.Delete(userID, postID); err != nil {
		switch {
		case errors.Is(err, service.ErrPostNotFound):
			response.NotFound(c, "帖子不存在")
		case errors.Is(err, service.ErrPermissionDenied):
			response.Forbidden(c, "无权删除此帖子")
		default:
			slog.Warn("post deletion failed", "user_id", userID, "post_id", postID, "err", err)
			response.InternalError(c, "帖子暂时无法删除，请稍后重试")
		}
		return
	}
	response.Success(c, nil)
}

func (h *PostHandler) GetByID(c *gin.Context) {
	postID := getInt64Param(c, "id")
	currentUserID := c.GetInt64("user_id")
	post, err := h.svc.GetByID(postID, currentUserID)
	if err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			response.NotFound(c, "帖子不存在")
		} else {
			slog.Warn("post lookup failed", "post_id", postID, "err", err)
			response.InternalError(c, "帖子暂时无法加载，请稍后重试")
		}
		return
	}
	response.Success(c, post)
}

func (h *PostHandler) RecordView(c *gin.Context) {
	postID := getInt64Param(c, "id")
	if postID <= 0 {
		response.BadRequest(c, "无效的帖子 ID")
		return
	}
	if err := h.svc.RecordView(postID, c.GetInt64("user_id")); err != nil {
		response.NotFound(c, "帖子不存在")
		return
	}
	response.Success(c, nil)
}

// --- Web page handlers ---

func (h *PostHandler) CreatePage(c *gin.Context) {
	c.HTML(http.StatusOK, "create_post.html", userData(c, gin.H{"title": "发布作品 - ShareO"}))
}

func (h *PostHandler) DetailPage(c *gin.Context) {
	postID := getInt64Param(c, "id")
	currentUserID := c.GetInt64("user_id")
	post, err := h.svc.GetByID(postID, currentUserID)
	if err != nil {
		c.HTML(http.StatusNotFound, "404.html", userData(c, gin.H{"title": "404"}))
		return
	}
	commentError := ""
	switch c.Query("error") {
	case "comment_invalid":
		commentError = "评论内容不能为空且不能超过 500 个字符。"
	case "post_not_found":
		commentError = "帖子不存在或已不可见。"
	case "comment_failed":
		commentError = "评论暂时无法提交，请稍后重试。"
	}
	c.HTML(http.StatusOK, "post_detail.html", userData(c, gin.H{
		"title":        post.Content,
		"Post":         post,
		"CommentError": commentError,
	}))
}

func (h *PostHandler) EditPage(c *gin.Context) {
	postID := getInt64Param(c, "id")
	currentUserID := c.GetInt64("user_id")
	post, err := h.svc.GetByID(postID, currentUserID)
	if err != nil || post.UserID != currentUserID {
		c.Redirect(http.StatusFound, "/")
		return
	}
	c.HTML(http.StatusOK, "edit_post.html", userData(c, gin.H{
		"title": "编辑帖子 - ShareO",
		"Post":  post,
	}))
}

// WebCreate handles form-based post creation
func (h *PostHandler) WebCreate(c *gin.Context) {
	userID := c.GetInt64("user_id")
	content := c.PostForm("content")
	// Accept comma-separated string (from JS join) or multiple form fields
	imageURLs := strings.Split(c.PostForm("images"), ",")
	if len(imageURLs) == 1 && imageURLs[0] == "" {
		imageURLs = nil
	}
	if len(imageURLs) == 0 {
		// Try single file upload
		_, err := c.FormFile("image")
		if err == nil {
			// Upload handled separately via /api/v1/upload
			c.HTML(http.StatusOK, "create_post.html", userData(c, gin.H{"title": "发布作品", "Error": "请先上传图片"}))
			return
		}
		c.HTML(http.StatusOK, "create_post.html", userData(c, gin.H{"title": "发布作品", "Error": "至少需要一张图片"}))
		return
	}

	req := service.CreatePostReq{
		Content: content,
		Images:  imageURLs,
	}
	_, err := h.svc.Create(userID, req)
	if err != nil {
		if !errors.Is(err, service.ErrPostInvalid) {
			slog.Warn("web post creation failed", "user_id", userID, "err", err)
		}
		message := "帖子暂时无法发布，请稍后重试"
		if errors.Is(err, service.ErrPostInvalid) {
			message = "帖子内容或图片参数无效"
		}
		c.HTML(http.StatusOK, "create_post.html", userData(c, gin.H{"title": "发布作品", "Error": message}))
		return
	}
	c.Redirect(http.StatusFound, "/")
}

func (h *PostHandler) WebUpdate(c *gin.Context) {
	userID := c.GetInt64("user_id")
	postID := getInt64Param(c, "id")
	content := c.PostForm("content")
	_, err := h.svc.Update(userID, postID, content)
	if err != nil {
		if !errors.Is(err, service.ErrPostInvalid) && !errors.Is(err, service.ErrPostNotFound) && !errors.Is(err, service.ErrPermissionDenied) {
			slog.Warn("web post update failed", "user_id", userID, "post_id", postID, "err", err)
		}
		message := "帖子暂时无法更新，请稍后重试"
		switch {
		case errors.Is(err, service.ErrPostInvalid):
			message = "帖子内容无效"
		case errors.Is(err, service.ErrPostNotFound):
			message = "帖子不存在"
		case errors.Is(err, service.ErrPermissionDenied):
			message = "无权编辑此帖子"
		}
		c.HTML(http.StatusOK, "edit_post.html", userData(c, gin.H{"Error": message}))
		return
	}
	c.Redirect(http.StatusFound, "/post/"+c.Param("id"))
}
