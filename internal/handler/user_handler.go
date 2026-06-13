package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/service"
)

type UserHandler struct {
	svc *service.UserService
}

func NewUserHandler() *UserHandler {
	return &UserHandler{svc: service.NewUserService()}
}

func (h *UserHandler) ProfilePage(c *gin.Context) {
	profileUserID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	currentUserID := c.GetInt64("user_id")

	data, err := h.svc.GetProfile(profileUserID, currentUserID)
	if err != nil || data.User == nil {
		c.HTML(http.StatusNotFound, "404.html", gin.H{"title": "用户不存在"})
		return
	}

	isOwnProfile := currentUserID == profileUserID

	c.HTML(http.StatusOK, "user_profile.html", userData(c, gin.H{
		"title":        data.User.Username + " - ShareO",
		"ProfileUser":  data.User,
		"Posts":        data.Posts,
		"TotalPosts":   data.TotalPosts,
		"PostCount":    data.PostCount,
		"Following":    data.Following,
		"Followers":    data.Followers,
		"IsOwnProfile": isOwnProfile,
		"IsFollowing":  data.IsFollowing,
	}))
}
