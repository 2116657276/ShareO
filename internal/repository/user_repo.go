package repository

import (
	"context"
	"errors"
	"log/slog"

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

func (r *UserRepo) GetSessionVersion(ctx context.Context, userID int64) (int64, error) {
	if DB == nil {
		return 0, errors.New("database not initialized")
	}
	var user model.User
	err := DB.WithContext(ctx).Select("session_version").First(&user, userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if user.SessionVersion <= 0 {
		return 1, nil
	}
	return user.SessionVersion, nil
}

func (r *UserRepo) BumpSessionVersion(ctx context.Context, userID int64) error {
	result := DB.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).
		UpdateColumn("session_version", gorm.Expr("session_version + 1"))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *UserRepo) UpdatePasswordAndBumpSession(ctx context.Context, userID int64, passwordHash string) error {
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
			"password_hash":   passwordHash,
			"session_version": gorm.Expr("session_version + 1"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
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
	result := DB.Model(&model.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":          status,
		"session_version": gorm.Expr("session_version + 1"),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *UserRepo) CountByRole(role string) int64 {
	var count int64
	if err := DB.Model(&model.User{}).Where("role = ?", role).Count(&count).Error; err != nil {
		slog.Warn("failed to count users by role", "role", role, "err", err)
	}
	return count
}

func (r *UserRepo) CountByStatus(status int8) int64 {
	var count int64
	if err := DB.Model(&model.User{}).Where("status = ?", status).Count(&count).Error; err != nil {
		slog.Warn("failed to count users by status", "status", status, "err", err)
	}
	return count
}

func (r *UserRepo) UpdateFields(id int64, updates map[string]interface{}) error {
	return DB.Model(&model.User{}).Where("id = ?", id).Updates(updates).Error
}

func (r *UserRepo) GetFollowCounts(userID int64) (following, followers int64) {
	if err := DB.Model(&model.Follow{}).Where("follower_id = ?", userID).Count(&following).Error; err != nil {
		slog.Warn("failed to count following", "user_id", userID, "err", err)
	}
	if err := DB.Model(&model.Follow{}).Where("followee_id = ?", userID).Count(&followers).Error; err != nil {
		slog.Warn("failed to count followers", "user_id", userID, "err", err)
	}
	return
}
