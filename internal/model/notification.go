package model

import "time"

// Notification types
const (
	NotifTypeLike    = "like"
	NotifTypeComment = "comment"
	NotifTypeFollow  = "follow"
	NotifTypeRepost  = "repost"
	NotifTypeReview  = "review"
)

type Notification struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"not null;index:idx_notif_user_status,priority:1" json:"user_id"`
	Type      string    `gorm:"type:varchar(20);not null;index:idx_notif_user_status,priority:2" json:"type"`
	ActorID   int64     `gorm:"not null" json:"actor_id"`
	TargetID  int64     `gorm:"default:0" json:"target_id"`
	IsRead    int8      `gorm:"default:0;index:idx_notif_user_status,priority:3" json:"is_read"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	Actor *User `gorm:"foreignKey:ActorID" json:"actor,omitempty"`
}

func (Notification) TableName() string { return "notifications" }
