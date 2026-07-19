package repository

import (
	"errors"
	"log"

	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm"
)

type CommentRepo struct{}

func NewCommentRepo() *CommentRepo { return &CommentRepo{} }

func (r *CommentRepo) CountNonDeleted() int64 {
	var count int64
	if err := DB.Model(&model.Comment{}).Where("is_deleted = 0").Count(&count).Error; err != nil {
		log.Printf("CommentRepo.CountNonDeleted: %v", err)
	}
	return count
}

func (r *CommentRepo) Create(comment *model.Comment) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(comment).Error; err != nil {
			return err
		}
		// Sync post comment_count via COUNT (idempotent, safe with triggers)
		if syncErr := tx.Model(&model.Post{}).Where("id = ?", comment.PostID).UpdateColumn("comment_count",
			gorm.Expr("(SELECT COUNT(*) FROM comments WHERE post_id = ? AND is_deleted = 0)", comment.PostID)).Error; syncErr != nil {
			log.Printf("CommentRepo: failed to sync comment_count for post %d: %v", comment.PostID, syncErr)
		}
		return nil
	})
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
	return DB.Transaction(func(tx *gorm.DB) error {
		// Find the comment first to get postID for count sync
		var comment model.Comment
		if err := tx.First(&comment, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return gorm.ErrRecordNotFound
			}
			return err
		}

		result := tx.Model(&model.Comment{}).Where("id = ? AND user_id = ?", id, userID).
			Update("is_deleted", 1)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		// Sync post comment_count via COUNT (idempotent, safe with triggers)
		if syncErr := tx.Model(&model.Post{}).Where("id = ?", comment.PostID).UpdateColumn("comment_count",
			gorm.Expr("(SELECT COUNT(*) FROM comments WHERE post_id = ? AND is_deleted = 0)", comment.PostID)).Error; syncErr != nil {
			log.Printf("CommentRepo: failed to sync comment_count for post %d: %v", comment.PostID, syncErr)
		}
		return nil
	})
}
