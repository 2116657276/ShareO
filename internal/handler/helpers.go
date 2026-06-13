package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

// userData adds user context from gin.Context into the template data map.
// Avatar is fetched once per request and cached in the gin context.
func userData(c *gin.Context, data gin.H) gin.H {
	if data == nil {
		data = gin.H{}
	}
	userID := c.GetInt64("user_id")
	if userID > 0 {
		data["user_id"] = userID
		data["username"] = c.GetString("username")
		data["role"] = c.GetString("role")

		// Cache avatar in context to avoid repeated DB queries within the same request
		avatarKey := "ctx_avatar_url"
		if avatar, exists := c.Get(avatarKey); exists {
			data["user_avatar"] = avatar.(string)
		} else {
			userRepo := repository.NewUserRepo()
			if user, err := userRepo.FindByID(userID); err == nil && user != nil {
				data["user_avatar"] = user.AvatarURL
				c.Set(avatarKey, user.AvatarURL)
			} else {
				data["user_avatar"] = ""
				c.Set(avatarKey, "")
			}
		}
	}
	return data
}

// --- Shared pagination helpers ---

// getPage reads the "page" query param, defaults to 1.
func getPage(c *gin.Context) int {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page <= 0 {
		page = 1
	}
	return page
}

// getPageSizePair reads "page" and "page_size" query params.
func getPageSizePair(c *gin.Context, defaultSize int) (page, pageSize int) {
	page = getPage(c)
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", strconv.Itoa(defaultSize)))
	if pageSize <= 0 || pageSize > 200 {
		pageSize = defaultSize
	}
	return
}

// calcPages returns the total number of pages given total items and page size.
func calcPages(total int64, pageSize int) int {
	if total <= 0 {
		return 0
	}
	return (int(total) + pageSize - 1) / pageSize
}

// respondPage writes a paginated success response.
func respondPage(c *gin.Context, page, pageSize int, list interface{}, total int64) {
	response.Success(c, response.PageResponse{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func getInt64Param(c *gin.Context, key string) int64 {
	v, _ := strconv.ParseInt(c.Param(key), 10, 64)
	return v
}
