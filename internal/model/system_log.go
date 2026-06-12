package model

import "time"

type SystemLog struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    *int64    `gorm:"index:idx_logs_user,priority:1" json:"user_id"`
	Action    string    `gorm:"type:varchar(255);not null;index:idx_logs_action,length:64" json:"action"`
	Detail    string    `gorm:"type:text" json:"detail"`
	IP        string    `gorm:"type:varchar(45);default:''" json:"ip"`
	CreatedAt time.Time `gorm:"autoCreateTime;index:idx_logs_time,priority:1,sort:desc" json:"created_at"`
}

func (SystemLog) TableName() string { return "system_logs" }
