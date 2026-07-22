package model

import "time"

// Conversation represents a private direct-message chat.
type Conversation struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	DmKey     string    `gorm:"type:varchar(50);uniqueIndex:uk_conversations_dm_key;not null" json:"dm_key"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// Non-persisted: populated by service layer for API responses
	Title       string               `gorm:"-" json:"title"`
	LastMessage *Message             `gorm:"-" json:"last_message,omitempty"`
	Members     []ConversationMember `gorm:"-" json:"members,omitempty"`
	UnreadCount int64                `gorm:"-" json:"unread_count"`
}

func (Conversation) TableName() string { return "conversations" }

// ConversationMember links one of the two users to a private conversation.
type ConversationMember struct {
	ID                int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID    int64     `gorm:"not null;uniqueIndex:idx_conv_user,priority:1" json:"conversation_id"`
	UserID            int64     `gorm:"not null;uniqueIndex:idx_conv_user,priority:2;index:idx_user_conv,priority:1" json:"user_id"`
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

type BotReply struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	SourceMessageID int64     `gorm:"not null;uniqueIndex:uk_bot_replies_source" json:"source_message_id"`
	ReplyMessageID  int64     `gorm:"not null;uniqueIndex:uk_bot_replies_reply" json:"reply_message_id"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (BotReply) TableName() string { return "bot_replies" }

type BotCitation struct {
	PostID  int64  `json:"post_id"`
	ChunkID string `json:"chunk_id"`
}

type MessageMeta struct {
	SourceMessageID int64         `json:"source_message_id,omitempty"`
	Citations       []BotCitation `json:"citations,omitempty"`
}
