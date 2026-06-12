package model

import "time"

type PostImage struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	PostID    int64     `gorm:"not null;index:idx_post_images_post,priority:1" json:"post_id"`
	ImageURL  string    `gorm:"type:varchar(500);not null" json:"image_url"`
	SortOrder int       `gorm:"default:0" json:"sort_order"`
	Width     int       `gorm:"default:0" json:"width"`
	Height    int       `gorm:"default:0" json:"height"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (PostImage) TableName() string { return "post_images" }
