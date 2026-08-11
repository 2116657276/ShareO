package repository

import (
	"context"
	"errors"
	"log/slog"

	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm"
)

type CommentRepo struct {
	likeRepo *CommentLikeRepo
}

func NewCommentRepo() *CommentRepo { return &CommentRepo{likeRepo: NewCommentLikeRepo()} }

func (r *CommentRepo) CountNonDeleted() int64 {
	var count int64
	if err := DB.Model(&model.Comment{}).Where("is_deleted = 0").Count(&count).Error; err != nil {
		slog.Warn("failed to count comments", "err", err)
	}
	return count
}

// HasUserCommented checks whether a user has commented on a given post.
func (r *CommentRepo) HasUserCommented(postID, userID int64) (bool, error) {
	var count int64
	err := DB.Model(&model.Comment{}).
		Where("post_id = ? AND user_id = ? AND is_deleted = 0", postID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *CommentRepo) Create(comment *model.Comment) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(comment).Error; err != nil {
			return err
		}
		// Keep the row and denormalized counter in the same Go transaction.
		if syncErr := tx.Model(&model.Post{}).Where("id = ?", comment.PostID).UpdateColumn("comment_count",
			gorm.Expr("(SELECT COUNT(*) FROM comments WHERE post_id = ? AND is_deleted = 0)", comment.PostID)).Error; syncErr != nil {
			return syncErr
		}
		return nil
	})
}

func (r *CommentRepo) FindByPostID(postID int64, page, pageSize int) ([]model.Comment, int64, error) {
	return r.FindByPostIDForUser(postID, page, pageSize, 0)
}

func (r *CommentRepo) FindByPostIDForUser(postID int64, page, pageSize int, currentUserID int64) ([]model.Comment, int64, error) {
	var comments []model.Comment
	var total int64

	baseQuery := DB.Model(&model.Comment{}).
		Joins("JOIN posts AS p ON p.id = comments.post_id").
		Where(
			"comments.post_id = ? AND comments.parent_id IS NULL AND comments.is_deleted = 0 AND p.is_deleted = 0 AND p.status = ?",
			postID,
			model.StatusApproved,
		)

	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := baseQuery.Preload("User").Preload("ReplyTo").
		Preload("Children", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_deleted = 0 AND post_id = ?", postID).
				Preload("User").Preload("ReplyTo").Order("created_at ASC")
		}).
		Order("created_at ASC").Offset(offset).Limit(pageSize).Find(&comments).Error
	if err != nil {
		return nil, 0, err
	}
	r.fillLikeState(comments, currentUserID)

	return comments, total, nil
}

// FindFeaturedByPostIDs loads all eligible top-level comments in one ordered
// query. The first row for each post is the deterministic high-heat comment;
// User is preloaded in one additional association query rather than per post.
func (r *CommentRepo) FindFeaturedByPostIDs(postIDs []int64, currentUserID int64) (map[int64]*model.Comment, error) {
	result := make(map[int64]*model.Comment)
	if len(postIDs) == 0 {
		return result, nil
	}
	var comments []model.Comment
	if err := DB.Model(&model.Comment{}).
		Joins("JOIN posts AS p ON p.id = comments.post_id").
		Where(
			"comments.post_id IN ? AND comments.parent_id IS NULL AND comments.is_deleted = 0 AND p.is_deleted = 0 AND p.status = ?",
			postIDs,
			model.StatusApproved,
		).
		Preload("User").
		Order("post_id ASC, like_count DESC, created_at DESC, id DESC").Find(&comments).Error; err != nil {
		return nil, err
	}
	for i := range comments {
		comment := comments[i]
		if _, exists := result[comment.PostID]; !exists {
			result[comment.PostID] = &comment
		}
	}
	featuredIDs := make([]int64, 0, len(result))
	for _, comment := range result {
		featuredIDs = append(featuredIDs, comment.ID)
	}
	likeRepo := r.likeRepo
	if likeRepo == nil {
		likeRepo = NewCommentLikeRepo()
	}
	liked := likeRepo.GetUserLikedCommentIDs(currentUserID, featuredIDs)
	for postID, comment := range result {
		comment.IsLiked = liked[comment.ID]
		result[postID] = comment
	}
	return result, nil
}

func (r *CommentRepo) FindVisibleByID(id int64) (*model.Comment, error) {
	var comment model.Comment
	err := DB.Table("comments AS c").
		Joins("JOIN posts AS p ON p.id = c.post_id").
		Select("c.id, c.post_id, c.user_id, c.parent_id, c.reply_to_uid, c.content, c.like_count, c.is_deleted, c.created_at").
		Where("c.id = ? AND c.is_deleted = 0 AND p.is_deleted = 0 AND p.status = ?", id, model.StatusApproved).
		Take(&comment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &comment, err
}

// FindVisibleTopLevelForPost validates the parent boundary before a reply is
// inserted. A reply may only point to a visible top-level comment on the same
// post; this keeps the comment tree and its denormalized counter consistent.
func (r *CommentRepo) FindVisibleTopLevelForPost(ctx context.Context, postID, commentID int64) (*model.Comment, error) {
	var comment model.Comment
	err := DB.WithContext(ctx).
		Where("id = ? AND post_id = ? AND parent_id IS NULL AND is_deleted = 0", commentID, postID).
		First(&comment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &comment, err
}

func (r *CommentRepo) fillLikeState(comments []model.Comment, currentUserID int64) {
	if currentUserID <= 0 || len(comments) == 0 {
		return
	}
	ids := make([]int64, 0, len(comments))
	for i := range comments {
		ids = append(ids, comments[i].ID)
		for _, child := range comments[i].Children {
			if child != nil {
				ids = append(ids, child.ID)
			}
		}
	}
	likeRepo := r.likeRepo
	if likeRepo == nil {
		likeRepo = NewCommentLikeRepo()
	}
	liked := likeRepo.GetUserLikedCommentIDs(currentUserID, ids)
	for i := range comments {
		comments[i].IsLiked = liked[comments[i].ID]
		for _, child := range comments[i].Children {
			if child != nil {
				child.IsLiked = liked[child.ID]
			}
		}
	}
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

		// New replies are restricted to top-level comments, but walk existing
		// descendants as well so old nested data cannot remain visible or inflate
		// the counter after a parent is deleted.
		frontier := []int64{id}
		for len(frontier) > 0 {
			var children []model.Comment
			if err := tx.Select("id").Where("parent_id IN ? AND is_deleted = 0", frontier).Find(&children).Error; err != nil {
				return err
			}
			if len(children) == 0 {
				break
			}
			childIDs := make([]int64, 0, len(children))
			for _, child := range children {
				childIDs = append(childIDs, child.ID)
			}
			if err := tx.Model(&model.Comment{}).Where("id IN ?", childIDs).Update("is_deleted", 1).Error; err != nil {
				return err
			}
			frontier = childIDs
		}

		// Keep the soft delete and denormalized counter in the same Go transaction.
		if syncErr := tx.Model(&model.Post{}).Where("id = ?", comment.PostID).UpdateColumn("comment_count",
			gorm.Expr("(SELECT COUNT(*) FROM comments WHERE post_id = ? AND is_deleted = 0)", comment.PostID)).Error; syncErr != nil {
			return syncErr
		}
		return nil
	})
}
