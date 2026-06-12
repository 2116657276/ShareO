package repository

import (
	"errors"

	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm"
)

type UserRepo struct{}

func NewUserRepo() *UserRepo { return &UserRepo{} }

func (r *UserRepo) Create(user *model.User) error {
	return DB.Create(user).Error
}

func (r *UserRepo) FindByID(id int64) (*model.User, error) {
	var user model.User
	err := DB.First(&user, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &user, err
}

func (r *UserRepo) FindByUsername(username string) (*model.User, error) {
	var user model.User
	err := DB.Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &user, err
}

func (r *UserRepo) Update(user *model.User) error {
	return DB.Save(user).Error
}

func (r *UserRepo) List(page, pageSize int, role string, status *int) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	query := DB.Model(&model.User{})
	if role != "" {
		query = query.Where("role = ?", role)
	}
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&users).Error
	return users, total, err
}

func (r *UserRepo) UpdateStatus(id int64, status int8) error {
	return DB.Model(&model.User{}).Where("id = ?", id).Update("status", status).Error
}

func (r *UserRepo) GetFollowCounts(userID int64) (following, followers int64) {
	DB.Model(&model.Follow{}).Where("follower_id = ?", userID).Count(&following)
	DB.Model(&model.Follow{}).Where("followee_id = ?", userID).Count(&followers)
	return
}
