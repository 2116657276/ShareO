package repository

import (
	"errors"
	"log"

	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm"
)

type FavoriteRepo struct{}

func NewFavoriteRepo() *FavoriteRepo { return &FavoriteRepo{} }

// Toggle returns true if favorited, false if unfavorited.
// All operations run within a transaction to ensure favorite record and post favorite_count stay consistent.
func (r *FavoriteRepo) Toggle(userID, postID int64) (bool, error) {
	var favorited bool
	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing model.Favorite
		err := tx.Where("user_id = ? AND post_id = ?", userID, postID).First(&existing).Error
		if err == nil {
			tx.Delete(&existing)
			// Sync post favorite_count via COUNT (idempotent, safe with triggers)
			if syncErr := tx.Model(&model.Post{}).Where("id = ?", postID).UpdateColumn("favorite_count",
				gorm.Expr("(SELECT COUNT(*) FROM favorites WHERE post_id = ?)", postID)).Error; syncErr != nil {
				log.Printf("FavoriteRepo.Toggle(unfavorite): failed to sync favorite_count for post %d: %v", postID, syncErr)
			}
			favorited = false
			return nil
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fav := model.Favorite{UserID: userID, PostID: postID}
			if createErr := tx.Create(&fav).Error; createErr != nil {
				return createErr
			}
			// Sync post favorite_count via COUNT (idempotent, safe with triggers)
			if syncErr := tx.Model(&model.Post{}).Where("id = ?", postID).UpdateColumn("favorite_count",
				gorm.Expr("(SELECT COUNT(*) FROM favorites WHERE post_id = ?)", postID)).Error; syncErr != nil {
				log.Printf("FavoriteRepo.Toggle(favorite): failed to sync favorite_count for post %d: %v", postID, syncErr)
			}
			favorited = true
			return nil
		}
		return err
	})
	return favorited, err
}

func (r *FavoriteRepo) GetUserFavoritedPostIDs(userID int64, postIDs []int64) map[int64]bool {
	if len(postIDs) == 0 {
		return map[int64]bool{}
	}
	var favs []model.Favorite
	DB.Where("user_id = ? AND post_id IN ?", userID, postIDs).Find(&favs)
	result := make(map[int64]bool, len(favs))
	for _, f := range favs {
		result[f.PostID] = true
	}
	return result
}

func (r *FavoriteRepo) IsFavorited(userID, postID int64) bool {
	var count int64
	if err := DB.Model(&model.Favorite{}).Where("user_id = ? AND post_id = ?", userID, postID).Count(&count).Error; err != nil {
		log.Printf("FavoriteRepo.IsFavorited(%d, %d): %v", userID, postID, err)
	}
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
		return []model.Post{}, total, nil
	}

	var posts []model.Post
	DB.Where("id IN ? AND is_deleted = 0 AND status = ?", postIDs, model.StatusApproved).
		Preload("User").Preload("Images").Find(&posts)
	return posts, total, nil
}
