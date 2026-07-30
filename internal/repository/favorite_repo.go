package repository

import (
	"context"
	"sort"

	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm/clause"
)

type FavoriteRepo struct{}

func NewFavoriteRepo() *FavoriteRepo { return &FavoriteRepo{} }

func (r *FavoriteRepo) Ensure(ctx context.Context, userID, postID int64) error {
	favorite := model.Favorite{UserID: userID, PostID: postID}
	return DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&favorite).Error
}

func (r *FavoriteRepo) Remove(ctx context.Context, userID, postID int64) error {
	return DB.WithContext(ctx).Where("user_id = ? AND post_id = ?", userID, postID).
		Delete(&model.Favorite{}).Error
}

func (r *FavoriteRepo) IsFavorited(ctx context.Context, userID, postID int64) bool {
	var count int64
	if userID <= 0 || postID <= 0 {
		return false
	}
	if err := DB.WithContext(ctx).Model(&model.Favorite{}).
		Where("user_id = ? AND post_id = ?", userID, postID).Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

func (r *FavoriteRepo) ListVisible(ctx context.Context, userID int64, page, pageSize int) ([]model.Post, int64, error) {
	var total int64
	var postIDs []int64
	query := DB.WithContext(ctx).Table("favorites AS f").
		Joins("JOIN posts AS p ON p.id = f.post_id").
		Where("f.user_id = ? AND p.is_deleted = 0 AND p.status = ?", userID, model.StatusApproved)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("f.created_at DESC, f.id DESC").
		Offset((page-1)*pageSize).Limit(pageSize).Pluck("f.post_id", &postIDs).Error; err != nil {
		return nil, 0, err
	}
	if len(postIDs) == 0 {
		return []model.Post{}, total, nil
	}
	var posts []model.Post
	if err := DB.WithContext(ctx).Where("id IN ? AND is_deleted = 0 AND status = ?", postIDs, model.StatusApproved).
		Preload("User").Preload("Images").Find(&posts).Error; err != nil {
		return nil, 0, err
	}
	order := make(map[int64]int, len(postIDs))
	for index, postID := range postIDs {
		order[postID] = index
	}
	sort.Slice(posts, func(i, j int) bool { return order[posts[i].ID] < order[posts[j].ID] })
	return posts, total, nil
}
