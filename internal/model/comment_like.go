package model

import "time"

// CommentLike records one user's like on one comment.
// The unique user/comment pair is enforced by the database migration.
type CommentLike struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"not null;index:idx_comment_likes_user,priority:1" json:"user_id"`
	CommentID int64     `gorm:"not null;index:idx_comment_likes_comment,priority:1" json:"comment_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (CommentLike) TableName() string { return "comment_likes" }
