package repository

import (
	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm"
)

type CommentRepo struct{}

func NewCommentRepo() *CommentRepo { return &CommentRepo{} }

func (r *CommentRepo) Create(comment *model.Comment) error {
	return DB.Create(comment).Error
}

func (r *CommentRepo) FindByPostID(postID int64, page, pageSize int) ([]model.Comment, int64, error) {
	var comments []model.Comment
	var total int64

	baseQuery := DB.Model(&model.Comment{}).Where("post_id = ? AND parent_id IS NULL AND is_deleted = 0", postID)

	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := baseQuery.Preload("User").Preload("ReplyTo").
		Preload("Children", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_deleted = 0").Preload("User").Preload("ReplyTo").Order("created_at ASC")
		}).
		Order("created_at ASC").Offset(offset).Limit(pageSize).Find(&comments).Error

	return comments, total, err
}

func (r *CommentRepo) SoftDelete(id, userID int64) error {
	result := DB.Model(&model.Comment{}).Where("id = ? AND user_id = ?", id, userID).
		Update("is_deleted", 1)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
