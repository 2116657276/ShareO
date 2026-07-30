package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm"
)

type FollowRepo struct{}

func NewFollowRepo() *FollowRepo { return &FollowRepo{} }

func (r *FollowRepo) Toggle(followerID, followeeID int64) (bool, error) {
	if followerID == followeeID {
		return false, errors.New("cannot follow yourself")
	}
	var existing model.Follow
	err := DB.Where("follower_id = ? AND followee_id = ?", followerID, followeeID).First(&existing).Error
	if err == nil {
		if delErr := DB.Delete(&existing).Error; delErr != nil {
			return false, delErr
		}
		return false, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		f := model.Follow{FollowerID: followerID, FolloweeID: followeeID}
		if createErr := DB.Create(&f).Error; createErr != nil {
			return false, createErr
		}
		return true, nil
	}
	return false, err
}

func (r *FollowRepo) IsFollowing(followerID, followeeID int64) bool {
	var count int64
	if err := DB.Model(&model.Follow{}).Where("follower_id = ? AND followee_id = ?", followerID, followeeID).Count(&count).Error; err != nil {
		slog.Warn("failed to query follow state", "follower_id", followerID, "followee_id", followeeID, "err", err)
	}
	return count > 0
}

func (r *FollowRepo) GetFollowing(userID int64, page, pageSize int) ([]model.User, int64, error) {
	var total int64
	if err := DB.Model(&model.Follow{}).Where("follower_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	var follows []model.Follow
	if err := DB.Where("follower_id = ?", userID).
		Order("created_at DESC, id DESC").Offset(offset).Limit(pageSize).Find(&follows).Error; err != nil {
		return nil, 0, err
	}

	if len(follows) == 0 {
		return []model.User{}, total, nil
	}
	followeeIDs := make([]int64, len(follows))
	for i, follow := range follows {
		followeeIDs[i] = follow.FolloweeID
	}

	var users []model.User
	if err := DB.Where("id IN ?", followeeIDs).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	byID := make(map[int64]model.User, len(users))
	for _, user := range users {
		byID[user.ID] = user
	}
	ordered := make([]model.User, 0, len(follows))
	for _, follow := range follows {
		user, ok := byID[follow.FolloweeID]
		if !ok {
			continue
		}
		followedAt := follow.CreatedAt
		user.FollowedAt = &followedAt
		ordered = append(ordered, user)
	}
	return ordered, total, nil
}

// GetFollowedAt returns follow creation times for the requested followees.
// It is used to order empty conversations without changing persisted chat data.
func (r *FollowRepo) GetFollowedAt(ctx context.Context, followerID int64, followeeIDs []int64) (map[int64]time.Time, error) {
	result := make(map[int64]time.Time)
	if followerID <= 0 || len(followeeIDs) == 0 {
		return result, nil
	}
	var follows []model.Follow
	if err := DB.WithContext(ctx).Where("follower_id = ? AND followee_id IN ?", followerID, followeeIDs).Find(&follows).Error; err != nil {
		return nil, err
	}
	for _, follow := range follows {
		result[follow.FolloweeID] = follow.CreatedAt
	}
	return result, nil
}

func (r *FollowRepo) GetFollowers(userID int64, page, pageSize int) ([]model.User, int64, error) {
	var total int64
	var followerIDs []int64

	DB.Model(&model.Follow{}).Where("followee_id = ?", userID).Count(&total)

	offset := (page - 1) * pageSize
	DB.Model(&model.Follow{}).Where("followee_id = ?", userID).
		Order("created_at DESC").Offset(offset).Limit(pageSize).Pluck("follower_id", &followerIDs)

	if len(followerIDs) == 0 {
		return []model.User{}, total, nil
	}

	var users []model.User
	DB.Where("id IN ?", followerIDs).Order(orderByField("id", followerIDs)).Find(&users)
	return users, total, nil
}

// orderByField builds an ORDER BY FIELD clause to preserve the order of IDs.
func orderByField(column string, ids []int64) string {
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%d", id)
	}
	return fmt.Sprintf("FIELD(%s, %s)", column, strings.Join(parts, ","))
}
