package repository

import (
	"errors"

	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CommentLikeRepo owns the comment_like row and the denormalized comment counter
// in the same transaction, mirroring the existing post like repository.
type CommentLikeRepo struct{}

func NewCommentLikeRepo() *CommentLikeRepo { return &CommentLikeRepo{} }

// Toggle returns the new like state and the authoritative counter value.
func (r *CommentLikeRepo) Toggle(userID, commentID int64) (bool, int, error) {
	var liked bool
	var likeCount int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var comment model.Comment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND is_deleted = 0", commentID).First(&comment).Error; err != nil {
			return err
		}

		var existing model.CommentLike
		err := tx.Where("user_id = ? AND comment_id = ?", userID, commentID).First(&existing).Error
		switch {
		case err == nil:
			if err := tx.Delete(&existing).Error; err != nil {
				return err
			}
			liked = false
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := tx.Create(&model.CommentLike{UserID: userID, CommentID: commentID}).Error; err != nil {
				return err
			}
			liked = true
		default:
			return err
		}

		if err := tx.Model(&model.Comment{}).Where("id = ?", commentID).
			UpdateColumn("like_count", gorm.Expr("(SELECT COUNT(*) FROM comment_likes WHERE comment_id = ?)", commentID)).Error; err != nil {
			return err
		}
		return tx.Model(&model.Comment{}).Where("id = ?", commentID).Pluck("like_count", &likeCount).Error
	})
	return liked, likeCount, err
}

func (r *CommentLikeRepo) GetUserLikedCommentIDs(userID int64, commentIDs []int64) map[int64]bool {
	result := make(map[int64]bool)
	if userID <= 0 || len(commentIDs) == 0 {
		return result
	}
	var likes []model.CommentLike
	if err := DB.Where("user_id = ? AND comment_id IN ?", userID, commentIDs).Find(&likes).Error; err != nil {
		return result
	}
	for _, like := range likes {
		result[like.CommentID] = true
	}
	return result
}
