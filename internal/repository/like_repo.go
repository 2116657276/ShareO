package repository

import (
	"errors"

	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm"
)

type LikeRepo struct{}

func NewLikeRepo() *LikeRepo { return &LikeRepo{} }

// Toggle returns true if liked, false if unliked
func (r *LikeRepo) Toggle(userID, postID int64) (bool, error) {
	var existing model.Like
	err := DB.Where("user_id = ? AND post_id = ?", userID, postID).First(&existing).Error
	if err == nil {
		// exists → unlike
		if delErr := DB.Delete(&existing).Error; delErr != nil {
			return false, delErr
		}
		return false, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// not exists → like
		like := model.Like{UserID: userID, PostID: postID}
		if createErr := DB.Create(&like).Error; createErr != nil {
			return false, createErr
		}
		return true, nil
	}
	return false, err
}

func (r *LikeRepo) IsLiked(userID, postID int64) bool {
	var count int64
	DB.Model(&model.Like{}).Where("user_id = ? AND post_id = ?", userID, postID).Count(&count)
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
		return nil, total, nil
	}

	var posts []model.Post
	DB.Where("id IN ? AND is_deleted = 0 AND status = ?", postIDs, "approved").
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
