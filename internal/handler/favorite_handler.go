package handler

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/service"
)

type FavoriteHandler struct {
	svc *service.FavoriteService
}

func NewFavoriteHandler() *FavoriteHandler {
	return &FavoriteHandler{svc: service.NewFavoriteService()}
}

func (h *FavoriteHandler) Ensure(c *gin.Context) {
	if err := h.svc.Ensure(c.Request.Context(), c.GetInt64("user_id"), getInt64Param(c, "id")); err != nil {
		if errors.Is(err, service.ErrFavoritePostNotFound) {
			response.NotFound(c, err.Error())
		} else {
			response.InternalError(c, "收藏操作失败")
		}
		return
	}
	response.Success(c, gin.H{"favorited": true})
}

func (h *FavoriteHandler) Remove(c *gin.Context) {
	if err := h.svc.Remove(c.Request.Context(), c.GetInt64("user_id"), getInt64Param(c, "id")); err != nil {
		if errors.Is(err, service.ErrFavoritePostNotFound) {
			response.NotFound(c, err.Error())
		} else {
			response.InternalError(c, "取消收藏失败")
		}
		return
	}
	response.Success(c, gin.H{"favorited": false})
}

func (h *FavoriteHandler) List(c *gin.Context) {
	page, pageSize := getPageSizePair(c, 12)
	posts, total, page, pageSize, err := h.svc.List(c.Request.Context(), c.GetInt64("user_id"), page, pageSize)
	if err != nil {
		response.InternalError(c, "收藏列表加载失败")
		return
	}
	response.Success(c, response.PageResponse{List: posts, Total: total, Page: page, PageSize: pageSize})
}
