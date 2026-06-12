package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/service"
)

type SocialHandler struct {
	svc *service.SocialService
}

func NewSocialHandler() *SocialHandler { return &SocialHandler{svc: service.NewSocialService()} }

// --- Like ---

func (h *SocialHandler) ToggleLike(c *gin.Context) {
	userID := c.GetInt64("user_id")
	postID := getInt64Param(c, "id")
	liked, err := h.svc.ToggleLike(userID, postID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"liked": liked})
}

// --- Favorite ---

func (h *SocialHandler) ToggleFavorite(c *gin.Context) {
	userID := c.GetInt64("user_id")
	postID := getInt64Param(c, "id")
	favorited, err := h.svc.ToggleFavorite(userID, postID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"favorited": favorited})
}

func (h *SocialHandler) GetFavorites(c *gin.Context) {
	userID := c.GetInt64("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "12"))
	posts, total, err := h.svc.GetFavorites(userID, page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, response.PageResponse{
		List: posts, Total: total, Page: page, PageSize: pageSize,
	})
}

// --- Follow ---

func (h *SocialHandler) ToggleFollow(c *gin.Context) {
	followerID := c.GetInt64("user_id")
	followeeID := getInt64Param(c, "id")
	following, err := h.svc.ToggleFollow(followerID, followeeID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"following": following})
}

func (h *SocialHandler) GetFollowing(c *gin.Context) {
	userID := getInt64Param(c, "id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	users, total, err := h.svc.GetFollowing(userID, page, 20)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"list": users, "total": total})
}

func (h *SocialHandler) GetFollowers(c *gin.Context) {
	userID := getInt64Param(c, "id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	users, total, err := h.svc.GetFollowers(userID, page, 20)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"list": users, "total": total})
}

// --- Comment ---

func (h *SocialHandler) CreateComment(c *gin.Context) {
	userID := c.GetInt64("user_id")
	postID := getInt64Param(c, "id")
	var req service.CreateCommentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	// Prefer URL post_id over body (Bug 4 fix: remove redundant post_id requirement)
	if req.PostID == 0 {
		req.PostID = postID
	}
	comment, err := h.svc.CreateComment(userID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, comment)
}

func (h *SocialHandler) GetComments(c *gin.Context) {
	postID := getInt64Param(c, "id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	comments, total, err := h.svc.GetComments(postID, page, 20)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"list": comments, "total": total})
}

func (h *SocialHandler) DeleteComment(c *gin.Context) {
	userID := c.GetInt64("user_id")
	commentID := getInt64Param(c, "cid")
	if err := h.svc.DeleteComment(userID, commentID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// --- Web Comment (form-based) ---

func (h *SocialHandler) WebCreateComment(c *gin.Context) {
	userID := c.GetInt64("user_id")
	postID := getInt64Param(c, "id")
	content := c.PostForm("content")
	_, err := h.svc.CreateComment(userID, service.CreateCommentReq{PostID: postID, Content: content})
	if err != nil {
		c.Redirect(http.StatusFound, "/post/"+c.Param("id")+"?error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/post/"+c.Param("id"))
}
