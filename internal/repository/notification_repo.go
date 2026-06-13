package repository

import (
	"github.com/zhoujianlin/ShareO/internal/model"
)

type NotificationRepo struct{}

func NewNotificationRepo() *NotificationRepo { return &NotificationRepo{} }

func (r *NotificationRepo) Create(notif *model.Notification) error {
	return DB.Create(notif).Error
}

func (r *NotificationRepo) List(userID int64, unreadOnly bool, page, pageSize int) ([]model.Notification, int64, error) {
	var list []model.Notification
	var total int64

	query := DB.Model(&model.Notification{}).Where("user_id = ?", userID)
	if unreadOnly {
		query = query.Where("is_read = 0")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Preload("Actor").Order("created_at DESC").
		Offset(offset).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (r *NotificationRepo) MarkRead(id, userID int64) error {
	return DB.Model(&model.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("is_read", 1).Error
}

func (r *NotificationRepo) MarkAllRead(userID int64) error {
	return DB.Model(&model.Notification{}).
		Where("user_id = ? AND is_read = 0", userID).
		Update("is_read", 1).Error
}

func (r *NotificationRepo) UnreadCount(userID int64) int64 {
	var count int64
	DB.Model(&model.Notification{}).Where("user_id = ? AND is_read = 0", userID).Count(&count)
	return count
}
