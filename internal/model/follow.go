package model

import "time"

type Follow struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	FollowerID int64     `gorm:"not null;uniqueIndex:uk_follow,priority:1;index:idx_follow_follower" json:"follower_id"`
	FolloweeID int64     `gorm:"not null;uniqueIndex:uk_follow,priority:2;index:idx_follow_followee" json:"followee_id"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (Follow) TableName() string { return "follows" }
