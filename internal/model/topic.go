package model

import "time"

type Topic struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	CoverImage  string    `gorm:"type:varchar(500);default:''" json:"cover_image"`
	StartTime   *time.Time `json:"start_time"`
	EndTime     *time.Time `json:"end_time"`
	PostCount   int       `gorm:"default:0" json:"post_count"`
	Status      int8      `gorm:"default:1;comment:1=进行中 0=已结束" json:"status"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (Topic) TableName() string { return "topics" }
