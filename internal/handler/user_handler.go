package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

type UserHandler struct {
	userRepo    *repository.UserRepo
	followRepo  *repository.FollowRepo
	postRepo    *repository.PostRepo
}

func NewUserHandler() *UserHandler {
	return &UserHandler{
		userRepo:   repository.NewUserRepo(),
		followRepo: repository.NewFollowRepo(),
		postRepo:   repository.NewPostRepo(),
	}
}

func (h *UserHandler) ProfilePage(c *gin.Context) {
	userID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	user, err := h.userRepo.FindByID(userID)
	if err != nil || user == nil {
		c.HTML(http.StatusNotFound, "404.html", gin.H{"title": "用户不存在"})
		return
	}

	currentUserID := c.GetInt64("user_id")
	isOwnProfile := currentUserID == userID
	isFollowing := false
	if currentUserID > 0 && !isOwnProfile {
		isFollowing = h.followRepo.IsFollowing(currentUserID, userID)
	}

	following, followers := h.userRepo.GetFollowCounts(userID)
	postCount := h.postRepo.CountByUser(userID)

	// Get user's posts
	q := repository.FeedQuery{
		UserID:   &userID,
		Status:   "approved",
		Sort:     "latest",
		Page:     1,
		PageSize: 12,
	}
	posts, total, _ := h.postRepo.Feed(q)

	c.HTML(http.StatusOK, "user_profile.html", userData(c, gin.H{
		"title":        user.Username + " - ShareO",
		"ProfileUser":  user,
		"Posts":        posts,
		"TotalPosts":   total,
		"PostCount":    postCount,
		"Following":    following,
		"Followers":    followers,
		"IsOwnProfile": isOwnProfile,
		"IsFollowing":  isFollowing,
})) 
}
