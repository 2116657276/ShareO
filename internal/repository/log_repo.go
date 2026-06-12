package repository

import "github.com/zhoujianlin/ShareO/internal/model"

type LogRepo struct{}

func NewLogRepo() *LogRepo { return &LogRepo{} }

func (r *LogRepo) Create(log *model.SystemLog) error {
	return DB.Create(log).Error
}

func (r *LogRepo) List(page, pageSize int, userID *int64, action string) ([]model.SystemLog, int64, error) {
	var logs []model.SystemLog
	var total int64

	query := DB.Model(&model.SystemLog{})
	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}
	if action != "" {
		query = query.Where("action = ?", action)
	}

	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}
