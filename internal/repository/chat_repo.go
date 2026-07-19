package repository

import (
	"context"
	"fmt"

	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm"
)

type ChatRepo struct {
	db *gorm.DB
}

// NewChatRepo creates a ChatRepo with injected DB for testability.
func NewChatRepo(db *gorm.DB) *ChatRepo {
	return &ChatRepo{db: db}
}

// --- Conversations ---

// EnsureDM finds or creates a DM conversation between two users.
// Uses dm_key unique constraint to handle concurrent creation safely —
// on duplicate key error, retries the SELECT.
func (r *ChatRepo) EnsureDM(ctx context.Context, user1ID, user2ID int64) (*model.Conversation, error) {
	dmKey := dmKey(user1ID, user2ID)

	// Try SELECT first (common case: DM already exists)
	var conv model.Conversation
	err := r.db.WithContext(ctx).Where("dm_key = ?", dmKey).First(&conv).Error
	if err == nil {
		return &conv, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	// Try INSERT — may fail with duplicate key if concurrent request won the race
	conv = model.Conversation{Type: model.ConvTypeDM, DmKey: &dmKey}
	if err := r.db.WithContext(ctx).Create(&conv).Error; err != nil {
		// Duplicate key: concurrent request created it first — retry SELECT
		if err := r.db.WithContext(ctx).Where("dm_key = ?", dmKey).First(&conv).Error; err != nil {
			return nil, err
		}
		return &conv, nil
	}

	// Successfully created — add both users as members
	members := []model.ConversationMember{
		{ConversationID: conv.ID, UserID: user1ID, Role: model.ConvRoleMember},
		{ConversationID: conv.ID, UserID: user2ID, Role: model.ConvRoleMember},
	}
	if err := r.db.WithContext(ctx).Create(&members).Error; err != nil {
		return nil, err
	}

	return &conv, nil
}

// CreateGroup creates a group conversation with the given owner and members.
func (r *ChatRepo) CreateGroup(ctx context.Context, ownerID int64, title string, memberIDs []int64) (*model.Conversation, error) {
	conv := model.Conversation{
		Type:    model.ConvTypeGroup,
		Title:   title,
		OwnerID: ownerID,
	}
	if err := r.db.WithContext(ctx).Create(&conv).Error; err != nil {
		return nil, err
	}

	// Add owner as member with owner role
	members := []model.ConversationMember{
		{ConversationID: conv.ID, UserID: ownerID, Role: model.ConvRoleOwner},
	}
	for _, uid := range memberIDs {
		if uid != ownerID {
			members = append(members, model.ConversationMember{
				ConversationID: conv.ID, UserID: uid, Role: model.ConvRoleMember,
			})
		}
	}
	if err := r.db.WithContext(ctx).Create(&members).Error; err != nil {
		return nil, err
	}

	return &conv, nil
}

// GetConversation fetches a conversation by ID.
func (r *ChatRepo) GetConversation(ctx context.Context, convID int64) (*model.Conversation, error) {
	var conv model.Conversation
	err := r.db.WithContext(ctx).First(&conv, convID).Error
	if err != nil {
		return nil, err
	}
	return &conv, nil
}

// ListConversations returns all conversations for a user, with last message and member info.
func (r *ChatRepo) ListConversations(ctx context.Context, userID int64) ([]model.Conversation, error) {
	// Get conversation IDs the user belongs to
	var memberConvs []model.ConversationMember
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&memberConvs).Error; err != nil {
		return nil, err
	}
	if len(memberConvs) == 0 {
		return []model.Conversation{}, nil
	}

	convIDs := make([]int64, len(memberConvs))
	for i, m := range memberConvs {
		convIDs[i] = m.ConversationID
	}

	var convs []model.Conversation
	if err := r.db.WithContext(ctx).
		Where("id IN ?", convIDs).
		Order("updated_at DESC").
		Find(&convs).Error; err != nil {
		return nil, err
	}

	return convs, nil
}

// --- Members ---

// IsMember returns true if the user is a member of the conversation.
func (r *ChatRepo) IsMember(ctx context.Context, convID, userID int64) bool {
	var count int64
	r.db.WithContext(ctx).Model(&model.ConversationMember{}).
		Where("conversation_id = ? AND user_id = ?", convID, userID).
		Count(&count)
	return count > 0
}

// GetMembers returns all members of a conversation.
func (r *ChatRepo) GetMembers(ctx context.Context, convID int64) ([]model.ConversationMember, error) {
	var members []model.ConversationMember
	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", convID).
		Preload("User").
		Find(&members).Error
	return members, err
}

// AddMember adds a user to a conversation.
func (r *ChatRepo) AddMember(ctx context.Context, convID, userID int64) error {
	m := model.ConversationMember{ConversationID: convID, UserID: userID, Role: model.ConvRoleMember}
	return r.db.WithContext(ctx).Create(&m).Error
}

// RemoveMember removes a user from a conversation.
func (r *ChatRepo) RemoveMember(ctx context.Context, convID, userID int64) error {
	return r.db.WithContext(ctx).
		Where("conversation_id = ? AND user_id = ?", convID, userID).
		Delete(&model.ConversationMember{}).Error
}

// --- Messages ---

// CreateMessage inserts a message and returns the created record (with auto-generated ID).
func (r *ChatRepo) CreateMessage(ctx context.Context, msg *model.Message) error {
	return r.db.WithContext(ctx).Create(msg).Error
}

// GetMessages returns messages using cursor-based pagination.
// beforeID=0 means "most recent first"; otherwise returns messages with id < beforeID.
func (r *ChatRepo) GetMessages(ctx context.Context, convID int64, beforeID int64, limit int) ([]model.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	var msgs []model.Message
	query := r.db.WithContext(ctx).
		Where("conversation_id = ?", convID).
		Preload("Sender").
		Order("id DESC").
		Limit(limit)
	if beforeID > 0 {
		query = query.Where("id < ?", beforeID)
	}
	err := query.Find(&msgs).Error
	return msgs, err
}

// GetLastMessage returns the most recent message in a conversation.
func (r *ChatRepo) GetLastMessage(ctx context.Context, convID int64) (*model.Message, error) {
	var msg model.Message
	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", convID).
		Order("id DESC").
		First(&msg).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &msg, nil
}

// GetRecentMessages returns the most recent N messages for RAG context (Phase 3).
func (r *ChatRepo) GetRecentMessages(ctx context.Context, convID int64, n int) ([]model.Message, error) {
	var msgs []model.Message
	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", convID).
		Preload("Sender").
		Order("id DESC").
		Limit(n).
		Find(&msgs).Error
	return msgs, err
}

// --- Read tracking ---

// UpdateReadMarker sets the user's last_read_message_id for a conversation.
func (r *ChatRepo) UpdateReadMarker(ctx context.Context, convID, userID, messageID int64) error {
	return r.db.WithContext(ctx).
		Model(&model.ConversationMember{}).
		Where("conversation_id = ? AND user_id = ?", convID, userID).
		Update("last_read_message_id", messageID).Error
}

// GetUnreadCount returns total unread message count across all conversations for a user.
func (r *ChatRepo) GetUnreadCount(ctx context.Context, userID int64) (int64, error) {
	// Get all memberships for this user
	var members []model.ConversationMember
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&members).Error; err != nil {
		return 0, err
	}
	if len(members) == 0 {
		return 0, nil
	}

	// For each membership, count messages with id > last_read_message_id
	var total int64
	for _, m := range members {
		var count int64
		r.db.WithContext(ctx).Model(&model.Message{}).
			Where("conversation_id = ? AND id > ?", m.ConversationID, m.LastReadMessageID).
			Count(&count)
		total += count
	}
	return total, nil
}

// GetLastReadMessageID returns the user's last_read_message_id for a conversation (0 if not a member).
func (r *ChatRepo) GetLastReadMessageID(ctx context.Context, convID, userID int64) (int64, error) {
	var member model.ConversationMember
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND user_id = ?", convID, userID).
		First(&member).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil
		}
		return 0, err
	}
	return member.LastReadMessageID, nil
}

// --- Helpers ---

// dmKey generates the canonical dm_key for two users: "smallerID:largerID".
func dmKey(a, b int64) string {
	if a < b {
		return fmt.Sprintf("%d:%d", a, b)
	}
	return fmt.Sprintf("%d:%d", b, a)
}
