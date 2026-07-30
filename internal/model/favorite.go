package model

import "time"

// Favorite is a private bookmark owned by one user.
type Favorite struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"not null;uniqueIndex:uk_favorites_user_post,priority:1;index:idx_favorites_user_created,priority:1" json:"user_id"`
	PostID    int64     `gorm:"not null;uniqueIndex:uk_favorites_user_post,priority:2;index:idx_favorites_post" json:"post_id"`
	CreatedAt time.Time `gorm:"autoCreateTime;index:idx_favorites_user_created,priority:2,sort:desc" json:"created_at"`
}

func (Favorite) TableName() string { return "favorites" }
