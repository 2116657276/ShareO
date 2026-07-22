package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChatRepo struct {
	db *gorm.DB
}

func NewChatRepo(db *gorm.DB) *ChatRepo { return &ChatRepo{db: db} }

func (r *ChatRepo) ValidateActiveUsers(ctx context.Context, userIDs []int64) error {
	if len(userIDs) == 0 {
		return nil
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.User{}).
		Where("id IN ? AND status = ?", userIDs, model.UserStatusActive).
		Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(userIDs)) {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *ChatRepo) EnsureDM(ctx context.Context, user1ID, user2ID int64) (*model.Conversation, error) {
	key := dmKey(user1ID, user2ID)
	conv := model.Conversation{DmKey: key}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// The duplicate branch returns the existing primary key through MySQL's
		// LAST_INSERT_ID. Unlike "insert-ignore then select", this waits for the
		// winning transaction and cannot observe a not-yet-committed gap.
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "dm_key"}},
			DoUpdates: clause.Assignments(map[string]any{
				"id": gorm.Expr("LAST_INSERT_ID(id)"),
			}),
		}).Create(&conv).Error; err != nil {
			return err
		}
		if err := tx.Where("dm_key = ?", key).First(&conv).Error; err != nil {
			return err
		}
		members := []model.ConversationMember{
			{ConversationID: conv.ID, UserID: user1ID},
			{ConversationID: conv.ID, UserID: user2ID},
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&members).Error
	})
	return &conv, err
}

func (r *ChatRepo) GetConversation(ctx context.Context, convID int64) (*model.Conversation, error) {
	var conv model.Conversation
	if err := r.db.WithContext(ctx).First(&conv, convID).Error; err != nil {
		return nil, err
	}
	return &conv, nil
}

func (r *ChatRepo) ListConversations(ctx context.Context, userID int64) ([]model.Conversation, error) {
	var conversations []model.Conversation
	err := r.db.WithContext(ctx).
		Table("conversations AS c").
		Select("c.*").
		Joins("JOIN conversation_members cm ON cm.conversation_id = c.id").
		Where("cm.user_id = ?", userID).
		Order("c.updated_at DESC, c.id DESC").
		Scan(&conversations).Error
	return conversations, err
}

func (r *ChatRepo) IsMember(ctx context.Context, convID, userID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.ConversationMember{}).
		Where("conversation_id = ? AND user_id = ?", convID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *ChatRepo) GetMembers(ctx context.Context, convID int64) ([]model.ConversationMember, error) {
	var members []model.ConversationMember
	err := r.db.WithContext(ctx).Where("conversation_id = ?", convID).
		Preload("User").Find(&members).Error
	return members, err
}

func (r *ChatRepo) CreateMessage(ctx context.Context, msg *model.Message) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(msg).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Conversation{}).Where("id = ?", msg.ConversationID).
			Update("updated_at", time.Now()).Error; err != nil {
			return err
		}
		result := tx.Model(&model.ConversationMember{}).
			Where("conversation_id = ? AND user_id = ?", msg.ConversationID, msg.SenderID).
			Update("last_read_message_id", gorm.Expr("GREATEST(last_read_message_id, ?)", msg.ID))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (r *ChatRepo) GetMessages(ctx context.Context, convID, beforeID, afterID int64, limit int) ([]model.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	var messages []model.Message
	query := r.db.WithContext(ctx).Where("conversation_id = ?", convID).Preload("Sender").Limit(limit)
	if afterID > 0 {
		query = query.Where("id > ?", afterID).Order("id ASC")
	} else {
		if beforeID > 0 {
			query = query.Where("id < ?", beforeID)
		}
		query = query.Order("id DESC")
	}
	return messages, query.Find(&messages).Error
}

func (r *ChatRepo) GetLastMessage(ctx context.Context, convID int64) (*model.Message, error) {
	var message model.Message
	err := r.db.WithContext(ctx).Where("conversation_id = ?", convID).
		Preload("Sender").Order("id DESC").First(&message).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &message, err
}

func (r *ChatRepo) GetRecentMessages(ctx context.Context, convID int64, n int) ([]model.Message, error) {
	var messages []model.Message
	err := r.db.WithContext(ctx).Where("conversation_id = ?", convID).
		Preload("Sender").Order("id DESC").Limit(n).Find(&messages).Error
	return messages, err
}

func (r *ChatRepo) UpdateReadMarker(ctx context.Context, convID, userID, messageID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var member model.ConversationMember
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("conversation_id = ? AND user_id = ?", convID, userID).
			First(&member).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.Message{}).
			Where("id = ? AND conversation_id = ?", messageID, convID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
		// RowsAffected may be zero when messageID is older than the current
		// marker. That is a successful idempotent call, not a missing member.
		return tx.Model(&model.ConversationMember{}).Where("id = ?", member.ID).
			Update("last_read_message_id", gorm.Expr("GREATEST(last_read_message_id, ?)", messageID)).Error
	})
}

func (r *ChatRepo) GetUnreadCount(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("messages AS m").
		Joins("JOIN conversation_members cm ON cm.conversation_id = m.conversation_id").
		Where("cm.user_id = ? AND m.id > cm.last_read_message_id AND m.sender_id <> ?", userID, userID).
		Count(&count).Error
	return count, err
}

func (r *ChatRepo) GetConversationUnreadCount(ctx context.Context, convID, userID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("messages AS m").
		Joins("JOIN conversation_members cm ON cm.conversation_id = m.conversation_id").
		Where("cm.conversation_id = ? AND cm.user_id = ? AND m.id > cm.last_read_message_id AND m.sender_id <> ?", convID, userID, userID).
		Count(&count).Error
	return count, err
}

func (r *ChatRepo) SearchActiveUsers(ctx context.Context, query string, excludeUserID int64, limit int) ([]model.User, error) {
	var users []model.User
	pattern := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(query) + "%"
	err := r.db.WithContext(ctx).Select("id", "username", "avatar_url").
		Where("status = ? AND id <> ? AND username LIKE ? ESCAPE '\\\\'", model.UserStatusActive, excludeUserID, pattern).
		Order("username ASC").Limit(limit).Find(&users).Error
	return users, err
}

func dmKey(a, b int64) string {
	if a < b {
		return fmt.Sprintf("%d:%d", a, b)
	}
	return fmt.Sprintf("%d:%d", b, a)
}
