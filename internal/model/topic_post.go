package model

import "time"

type TopicPost struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TopicID   int64     `gorm:"not null;uniqueIndex:uk_topic_post,priority:1" json:"topic_id"`
	PostID    int64     `gorm:"not null;uniqueIndex:uk_topic_post,priority:2;index:idx_tp_post" json:"post_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (TopicPost) TableName() string { return "topic_posts" }
