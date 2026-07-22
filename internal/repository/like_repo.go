package repository

import (
	"errors"
	"log/slog"
	"sort"

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
			// Sync post like_count via COUNT so duplicate/retried toggles converge.
			if syncErr := tx.Model(&model.Post{}).Where("id = ?", postID).UpdateColumn("like_count",
				gorm.Expr("(SELECT COUNT(*) FROM likes WHERE post_id = ?)", postID)).Error; syncErr != nil {
				return syncErr
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
			// Sync post like_count via COUNT so duplicate/retried toggles converge.
			if syncErr := tx.Model(&model.Post{}).Where("id = ?", postID).UpdateColumn("like_count",
				gorm.Expr("(SELECT COUNT(*) FROM likes WHERE post_id = ?)", postID)).Error; syncErr != nil {
				return syncErr
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
		slog.Warn("failed to count likes", "err", err)
	}
	return count
}

func (r *LikeRepo) IsLiked(userID, postID int64) bool {
	var count int64
	if err := DB.Model(&model.Like{}).Where("user_id = ? AND post_id = ?", userID, postID).Count(&count).Error; err != nil {
		slog.Warn("failed to query like state", "user_id", userID, "post_id", postID, "err", err)
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

	visibleLikes := DB.Table("likes AS l").
		Joins("JOIN posts AS p ON p.id = l.post_id").
		Where("l.user_id = ? AND p.is_deleted = 0 AND p.status = ?", userID, model.StatusApproved)
	if err := visibleLikes.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := visibleLikes.Order("l.created_at DESC").Offset(offset).Limit(pageSize).
		Pluck("l.post_id", &postIDs).Error; err != nil {
		return nil, 0, err
	}

	if len(postIDs) == 0 {
		return []model.Post{}, total, nil
	}

	var posts []model.Post
	if err := DB.Where("id IN ? AND is_deleted = 0 AND status = ?", postIDs, model.StatusApproved).
		Preload("User").Preload("Images").Find(&posts).Error; err != nil {
		return nil, 0, err
	}

	// preserve order from likes
	orderMap := make(map[int64]int, len(postIDs))
	for i, pid := range postIDs {
		orderMap[pid] = i
	}
	sort.Slice(posts, func(i, j int) bool { return orderMap[posts[i].ID] < orderMap[posts[j].ID] })
	return posts, total, nil
}
