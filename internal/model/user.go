package model

import "time"

type User struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string    `gorm:"type:varchar(50);uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"type:varchar(255);not null" json:"-"`
	Email        string    `gorm:"type:varchar(100);default:''" json:"email"`
	AvatarURL    string    `gorm:"type:varchar(500);default:''" json:"avatar_url"`
	Bio          string    `gorm:"type:varchar(200);default:''" json:"bio"`
	Role         string    `gorm:"type:enum('user','admin');default:'user'" json:"role"`
	Status       int8      `gorm:"default:1;comment:1=正常 0=封禁" json:"status"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// Associations
	Posts    []Post    `gorm:"foreignKey:UserID" json:"posts,omitempty"`
	Comments []Comment `gorm:"foreignKey:UserID" json:"-"`
}

func (User) TableName() string { return "users" }
