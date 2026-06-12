package model

import "time"

type Like struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"not null;uniqueIndex:uk_user_post,priority:1" json:"user_id"`
	PostID    int64     `gorm:"not null;uniqueIndex:uk_user_post,priority:2;index:idx_likes_post" json:"post_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (Like) TableName() string { return "likes" }
