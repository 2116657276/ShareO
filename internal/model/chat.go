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

// BotTaskOutbox stores a bot task beside the user message. Redis is only the
// delivery mechanism; a pending row is the durable source for replay after a
// publish failure or process restart.
type BotTaskOutbox struct {
	ID             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID int64      `gorm:"not null;index:idx_bot_outbox_pending,priority:1" json:"conversation_id"`
	MessageID      int64      `gorm:"not null;uniqueIndex:uk_bot_outbox_message" json:"message_id"`
	Status         string     `gorm:"type:enum('pending','published');not null;default:'pending';index:idx_bot_outbox_pending,priority:2" json:"status"`
	Attempts       int        `gorm:"not null;default:0" json:"attempts"`
	NextAttemptAt  time.Time  `gorm:"not null;index:idx_bot_outbox_pending,priority:3" json:"next_attempt_at"`
	LastError      string     `gorm:"type:varchar(500);not null;default:''" json:"last_error"`
	PublishedAt    *time.Time `json:"published_at,omitempty"`
	CreatedAt      time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (BotTaskOutbox) TableName() string { return "bot_task_outbox" }

type BotCitation struct {
	PostID  int64            `json:"post_id"`
	ChunkID string           `json:"chunk_id"`
	Preview *CitationPreview `json:"preview,omitempty"`
}

// CitationPreview is attached to authenticated chat responses after the
// citation has passed the approved/non-deleted visibility check. It is not
// written by the AI service and is intentionally a small response DTO.
type CitationPreview struct {
	ImageURL string         `json:"image_url,omitempty"`
	Content  string         `json:"content,omitempty"`
	Author   CitationAuthor `json:"author,omitempty"`
}

type CitationAuthor struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

type AIMode string

const (
	AIModeRAG   AIMode = "rag"
	AIModeAgent AIMode = "agent"
)

type AgentTraceStep struct {
	Index       int    `json:"index"`
	Tool        string `json:"tool"`
	Status      string `json:"status"`
	ResultCount int    `json:"result_count"`
	DurationMS  int64  `json:"duration_ms"`
}

type AgentTrace struct {
	Version           string           `json:"version"`
	Status            string           `json:"status"`
	StopReason        string           `json:"stop_reason"`
	FailureCategory   string           `json:"failure_category,omitempty"`
	ProviderAttempts  int              `json:"provider_attempts"`
	ProviderRetries   int              `json:"provider_retries"`
	RejectedToolCalls int              `json:"rejected_tool_calls"`
	TotalDurationMS   int64            `json:"total_duration_ms"`
	Steps             []AgentTraceStep `json:"steps"`
}

type BotReplyMeta struct {
	Mode  AIMode      `json:"ai_mode,omitempty"`
	Trace *AgentTrace `json:"agent_trace,omitempty"`
}

type MessageMeta struct {
	SourceMessageID int64         `json:"source_message_id,omitempty"`
	Citations       []BotCitation `json:"citations,omitempty"`
	Mode            AIMode        `json:"ai_mode,omitempty"`
	AgentTrace      *AgentTrace   `json:"agent_trace,omitempty"`
}
