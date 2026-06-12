package model

import "time"

type Comment struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	PostID      int64     `gorm:"not null;index:idx_comments_post,priority:1" json:"post_id"`
	UserID      int64     `gorm:"not null;index:idx_comments_user,priority:1" json:"user_id"`
	ParentID    *int64    `gorm:"index:idx_comments_parent" json:"parent_id"`
	ReplyToUID  *int64    `json:"reply_to_uid"`
	Content     string    `gorm:"type:text;not null" json:"content"`
	LikeCount   int       `gorm:"default:0" json:"like_count"`
	IsDeleted   int8      `gorm:"default:0" json:"is_deleted"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Associations
	User     *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	ReplyTo  *User      `gorm:"foreignKey:ReplyToUID" json:"reply_to,omitempty"`
	Children []*Comment `gorm:"foreignKey:ParentID" json:"children,omitempty"`
}

func (Comment) TableName() string { return "comments" }
