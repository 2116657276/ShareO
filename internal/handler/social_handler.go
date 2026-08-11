package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"

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
		if errors.Is(err, service.ErrPostNotFound) {
			response.NotFound(c, "帖子不存在或已不可见")
		} else {
			slog.Warn("post like toggle failed", "user_id", userID, "post_id", postID, "err", err)
			response.InternalError(c, "点赞操作暂时失败，请稍后重试")
		}
		return
	}
	response.Success(c, gin.H{"liked": liked})
}

func (h *SocialHandler) GetLikes(c *gin.Context) {
	userID := c.GetInt64("user_id")
	h.getLikesByUser(c, userID)
}

func (h *SocialHandler) GetUserLikes(c *gin.Context) {
	h.getLikesByUser(c, getInt64Param(c, "id"))
}

func (h *SocialHandler) getLikesByUser(c *gin.Context, userID int64) {
	page, pageSize := getPageSizePair(c, 12)
	posts, total, err := h.svc.GetLikedPosts(userID, page, pageSize)
	if err != nil {
		slog.Warn("liked posts lookup failed", "user_id", userID, "err", err)
		response.InternalError(c, "点赞列表暂时无法加载，请稍后重试")
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
		if errors.Is(err, service.ErrFollowSelf) {
			response.BadRequest(c, "cannot follow yourself")
			return
		}
		slog.Warn("follow toggle failed", "follower_id", followerID, "followee_id", followeeID, "err", err)
		response.InternalError(c, "关注操作暂时失败，请稍后重试")
		return
	}
	response.Success(c, gin.H{"following": following})
}

func (h *SocialHandler) GetFollowing(c *gin.Context) {
	userID := getInt64Param(c, "id")
	page, pageSize := getPageSizePair(c, 20)
	users, total, err := h.svc.GetFollowing(userID, page, pageSize)
	if err != nil {
		slog.Warn("following list failed", "user_id", userID, "err", err)
		response.InternalError(c, "关注列表暂时无法加载，请稍后重试")
		return
	}
	response.Success(c, gin.H{"list": users, "total": total})
}

func (h *SocialHandler) GetFollowers(c *gin.Context) {
	userID := getInt64Param(c, "id")
	page, pageSize := getPageSizePair(c, 20)
	users, total, err := h.svc.GetFollowers(userID, page, pageSize)
	if err != nil {
		slog.Warn("followers list failed", "user_id", userID, "err", err)
		response.InternalError(c, "粉丝列表暂时无法加载，请稍后重试")
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
		response.BadRequest(c, "评论请求格式无效")
		return
	}
	// Always use URL post_id (prevents cross-post comment injection)
	req.PostID = postID
	comment, err := h.svc.CreateComment(c.Request.Context(), userID, req)
	if err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			response.NotFound(c, err.Error())
		} else if errors.Is(err, service.ErrCommentInvalid) {
			response.BadRequest(c, "评论内容不能为空且不能超过 500 个字符")
		} else if errors.Is(err, service.ErrCommentParentInvalid) || errors.Is(err, service.ErrCommentReplyInvalid) {
			response.BadRequest(c, "评论回复关系无效")
		} else {
			slog.Warn("comment creation failed", "user_id", userID, "post_id", postID, "err", err)
			response.InternalError(c, "评论暂时无法提交，请稍后重试")
		}
		return
	}
	response.Success(c, comment)
}
func (h *SocialHandler) GetComments(c *gin.Context) {
	postID := getInt64Param(c, "id")
	page, pageSize := getPageSizePair(c, 20)
	comments, total, err := h.svc.GetCommentsForUser(postID, page, pageSize, c.GetInt64("user_id"))
	if err != nil {
		slog.Warn("comment list failed", "post_id", postID, "err", err)
		response.InternalError(c, "评论暂时无法加载，请稍后重试")
		return
	}
	response.Success(c, gin.H{"list": comments, "total": total})
}

func (h *SocialHandler) ToggleCommentLike(c *gin.Context) {
	liked, likeCount, err := h.svc.ToggleCommentLike(c.GetInt64("user_id"), getInt64Param(c, "cid"))
	if err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			response.NotFound(c, "评论不存在")
		} else {
			response.BadRequest(c, "评论点赞操作失败")
		}
		return
	}
	response.Success(c, gin.H{"liked": liked, "like_count": likeCount})
}

func (h *SocialHandler) DeleteComment(c *gin.Context) {
	userID := c.GetInt64("user_id")
	commentID := getInt64Param(c, "cid")
	if err := h.svc.DeleteComment(userID, commentID); err != nil {
		response.BadRequest(c, "评论不存在或无权删除")
		return
	}
	response.Success(c, nil)
}

// --- Web Comment (form-based) ---

func (h *SocialHandler) WebCreateComment(c *gin.Context) {
	userID := c.GetInt64("user_id")
	postID := getInt64Param(c, "id")
	content := c.PostForm("content")
	_, err := h.svc.CreateComment(c.Request.Context(), userID, service.CreateCommentReq{PostID: postID, Content: content})
	if err != nil {
		code := "comment_failed"
		switch {
		case errors.Is(err, service.ErrPostNotFound):
			code = "post_not_found"
		case errors.Is(err, service.ErrCommentInvalid):
			code = "comment_invalid"
		case errors.Is(err, service.ErrCommentParentInvalid), errors.Is(err, service.ErrCommentReplyInvalid):
			code = "comment_invalid"
		default:
			slog.Warn("web comment creation failed", "user_id", userID, "post_id", postID, "err", err)
		}
		c.Redirect(http.StatusFound, "/post/"+c.Param("id")+"?error="+url.QueryEscape(code))
		return
	}
	c.Redirect(http.StatusFound, "/post/"+c.Param("id"))
}
