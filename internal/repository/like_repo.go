package repository

import (
	"errors"
	"log"

	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm"
)

type LikeRepo struct{}

func NewLikeRepo() *LikeRepo { return &LikeRepo{} }

// Toggle returns true if liked, false if unliked.
// All operations run within a transaction to ensure like record and post like_count stay consistent.
func (r *LikeRepo) Toggle(userID, postID int64) (bool, error) {
	var liked bool
	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing model.Like
		err := tx.Where("user_id = ? AND post_id = ?", userID, postID).First(&existing).Error
		if err == nil {
			// exists → unlike
			if delErr := tx.Delete(&existing).Error; delErr != nil {
				return delErr
			}
			// Sync post like_count via COUNT (idempotent, safe with triggers)
			if syncErr := tx.Model(&model.Post{}).Where("id = ?", postID).UpdateColumn("like_count",
				gorm.Expr("(SELECT COUNT(*) FROM likes WHERE post_id = ?)", postID)).Error; syncErr != nil {
				log.Printf("LikeRepo.Toggle(unlike): failed to sync like_count for post %d: %v", postID, syncErr)
			}
			liked = false
			return nil
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// not exists → like
			like := model.Like{UserID: userID, PostID: postID}
			if createErr := tx.Create(&like).Error; createErr != nil {
				return createErr
			}
			// Sync post like_count via COUNT (idempotent, safe with triggers)
			if syncErr := tx.Model(&model.Post{}).Where("id = ?", postID).UpdateColumn("like_count",
				gorm.Expr("(SELECT COUNT(*) FROM likes WHERE post_id = ?)", postID)).Error; syncErr != nil {
				log.Printf("LikeRepo.Toggle(like): failed to sync like_count for post %d: %v", postID, syncErr)
			}
			liked = true
			return nil
		}
		return err
	})
	return liked, err
}

func (r *LikeRepo) CountTotal() int64 {
	var count int64
	if err := DB.Model(&model.Like{}).Count(&count).Error; err != nil {
		log.Printf("LikeRepo.CountTotal: %v", err)
	}
	return count
}

func (r *LikeRepo) IsLiked(userID, postID int64) bool {
	var count int64
	if err := DB.Model(&model.Like{}).Where("user_id = ? AND post_id = ?", userID, postID).Count(&count).Error; err != nil {
		log.Printf("LikeRepo.IsLiked(%d, %d): %v", userID, postID, err)
	}
	return count > 0
}

func (r *LikeRepo) GetUserLikedPostIDs(userID int64, postIDs []int64) map[int64]bool {
	if len(postIDs) == 0 {
		return map[int64]bool{}
	}
	var likes []model.Like
	DB.Where("user_id = ? AND post_id IN ?", userID, postIDs).Find(&likes)
	result := make(map[int64]bool, len(likes))
	for _, l := range likes {
		result[l.PostID] = true
	}
	return result
}

func (r *LikeRepo) GetUserLikedPosts(userID int64, page, pageSize int) ([]model.Post, int64, error) {
	var total int64
	var postIDs []int64

	DB.Model(&model.Like{}).Where("user_id = ?", userID).Count(&total)

	offset := (page - 1) * pageSize
	DB.Model(&model.Like{}).Where("user_id = ?", userID).
		Order("created_at DESC").Offset(offset).Limit(pageSize).Pluck("post_id", &postIDs)

	if len(postIDs) == 0 {
		return []model.Post{}, total, nil
	}

	var posts []model.Post
	DB.Where("id IN ? AND is_deleted = 0 AND status = ?", postIDs, model.StatusApproved).
		Preload("User").Preload("Images").Find(&posts)

	// preserve order from likes
	orderMap := make(map[int64]int, len(postIDs))
	for i, pid := range postIDs {
		orderMap[pid] = i
	}
	sorted := make([]model.Post, 0, len(posts))
	for _, p := range posts {
		if _, ok := orderMap[p.ID]; ok {
			sorted = append(sorted, p)
		}
	}
	return sorted, total, nil
}
