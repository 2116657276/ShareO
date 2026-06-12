package repository

import (
	"errors"

	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm"
)

type FavoriteRepo struct{}

func NewFavoriteRepo() *FavoriteRepo { return &FavoriteRepo{} }

func (r *FavoriteRepo) Toggle(userID, postID int64) (bool, error) {
	var existing model.Favorite
	err := DB.Where("user_id = ? AND post_id = ?", userID, postID).First(&existing).Error
	if err == nil {
		DB.Delete(&existing)
		return false, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		fav := model.Favorite{UserID: userID, PostID: postID}
		if createErr := DB.Create(&fav).Error; createErr != nil {
			return false, createErr
		}
		return true, nil
	}
	return false, err
}

func (r *FavoriteRepo) IsFavorited(userID, postID int64) bool {
	var count int64
	DB.Model(&model.Favorite{}).Where("user_id = ? AND post_id = ?", userID, postID).Count(&count)
	return count > 0
}

func (r *FavoriteRepo) GetUserFavorites(userID int64, page, pageSize int) ([]model.Post, int64, error) {
	var total int64
	DB.Model(&model.Favorite{}).Where("user_id = ?", userID).Count(&total)

	offset := (page - 1) * pageSize
	var favs []model.Favorite
	DB.Where("user_id = ?", userID).Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&favs)

	postIDs := make([]int64, len(favs))
	for i, f := range favs {
		postIDs[i] = f.PostID
	}
	if len(postIDs) == 0 {
		return nil, total, nil
	}

	var posts []model.Post
	DB.Where("id IN ? AND is_deleted = 0 AND status = ?", postIDs, "approved").
		Preload("User").Preload("Images").Find(&posts)
	return posts, total, nil
}
