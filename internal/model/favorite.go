package model

import "time"

type Favorite struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"not null;uniqueIndex:uk_user_fav,priority:1" json:"user_id"`
	PostID    int64     `gorm:"not null;uniqueIndex:uk_user_fav,priority:2" json:"post_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (Favorite) TableName() string { return "favorites" }
