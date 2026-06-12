package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

// userData adds user context from gin.Context into the template data map
func userData(c *gin.Context, data gin.H) gin.H {
	if data == nil {
		data = gin.H{}
	}
	userID := c.GetInt64("user_id")
	if userID > 0 {
		data["user_id"] = userID
		data["username"] = c.GetString("username")
		data["role"] = c.GetString("role")
		// fetch avatar on demand
		userRepo := repository.NewUserRepo()
		if user, err := userRepo.FindByID(userID); err == nil && user != nil {
			data["user_avatar"] = user.AvatarURL
		} else {
			data["user_avatar"] = ""
		}
	}
	return data
}
