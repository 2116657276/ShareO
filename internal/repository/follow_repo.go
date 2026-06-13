package repository

import (
	"errors"

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
		DB.Delete(&existing)
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
	DB.Model(&model.Follow{}).Where("follower_id = ? AND followee_id = ?", followerID, followeeID).Count(&count)
	return count > 0
}

func (r *FollowRepo) GetFollowing(userID int64, page, pageSize int) ([]model.User, int64, error) {
	var total int64
	var followeeIDs []int64

	DB.Model(&model.Follow{}).Where("follower_id = ?", userID).Count(&total)

	offset := (page - 1) * pageSize
	DB.Model(&model.Follow{}).Where("follower_id = ?", userID).
		Order("created_at DESC").Offset(offset).Limit(pageSize).Pluck("followee_id", &followeeIDs)

	if len(followeeIDs) == 0 {
		return []model.User{}, total, nil
	}

	var users []model.User
	DB.Where("id IN ?", followeeIDs).Find(&users)
	return users, total, nil
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
	DB.Where("id IN ?", followerIDs).Find(&users)
	return users, total, nil
}
