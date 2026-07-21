package model

import "time"

// Conversation types
const (
	ConvTypeDM    = "dm"
	ConvTypeGroup = "group"
)

// Conversation roles
const (
	ConvRoleOwner  = "owner"
	ConvRoleMember = "member"
)

// Conversation represents a DM or group chat.
type Conversation struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Type      string    `gorm:"type:varchar(10);not null;default:'dm'" json:"type"`
	Title     string    `gorm:"type:varchar(100);default:''" json:"title"`
	OwnerID   *int64    `gorm:"default:null" json:"owner_id"`
	DmKey     *string   `gorm:"type:varchar(50);uniqueIndex:idx_dm_key" json:"dm_key"` // nullable; NULLs don't conflict in unique index
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// Non-persisted: populated by service layer for API responses
	LastMessage *Message             `gorm:"-" json:"last_message,omitempty"`
	Members     []ConversationMember `gorm:"-" json:"members,omitempty"`
	UnreadCount int64                `gorm:"-" json:"unread_count"`
}

func (Conversation) TableName() string { return "conversations" }

// ConversationMember links a user to a conversation with a role.
type ConversationMember struct {
	ID                int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID    int64     `gorm:"not null;uniqueIndex:idx_conv_user,priority:1" json:"conversation_id"`
	UserID            int64     `gorm:"not null;uniqueIndex:idx_conv_user,priority:2;index:idx_user_conv,priority:1" json:"user_id"`
	Role              string    `gorm:"type:varchar(10);default:'member'" json:"role"`
	LastReadMessageID int64     `gorm:"not null;default:0" json:"last_read_message_id"`
	JoinedAt          time.Time `gorm:"autoCreateTime" json:"joined_at"`

	// Association
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (ConversationMember) TableName() string { return "conversation_members" }

// Message represents a chat message.
type Message struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID int64     `gorm:"not null;index:idx_conv_msg,priority:1" json:"conversation_id"`
	SenderID       int64     `gorm:"not null" json:"sender_id"`
	Content        string    `gorm:"type:text;not null" json:"content"`
	Meta           *string   `gorm:"type:json;default:null" json:"meta,omitempty"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Association
	Sender *User `gorm:"foreignKey:SenderID" json:"sender,omitempty"`
}

func (Message) TableName() string { return "messages" }
