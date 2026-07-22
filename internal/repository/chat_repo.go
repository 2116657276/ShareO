package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChatRepo struct {
	db *gorm.DB
}

var ErrInvalidBotConversation = errors.New("invalid bot conversation")

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

func (r *ChatRepo) GetMessageByID(ctx context.Context, messageID int64) (*model.Message, error) {
	var message model.Message
	err := r.db.WithContext(ctx).Preload("Sender").First(&message, messageID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &message, err
}

func (r *ChatRepo) VisiblePostIDs(ctx context.Context, postIDs []int64) (map[int64]struct{}, error) {
	visible := make(map[int64]struct{})
	if len(postIDs) == 0 {
		return visible, nil
	}
	var ids []int64
	err := r.db.WithContext(ctx).Model(&model.Post{}).
		Where("id IN ? AND status = ? AND is_deleted = 0", postIDs, model.StatusApproved).
		Pluck("id", &ids).Error
	for _, id := range ids {
		visible[id] = struct{}{}
	}
	return visible, err
}

func validChunkID(postID int64, chunkID string) bool {
	prefix := strconv.FormatInt(postID, 10) + ":"
	if !strings.HasPrefix(chunkID, prefix) {
		return false
	}
	suffix := strings.TrimPrefix(chunkID, prefix)
	chunkNo, err := strconv.ParseInt(suffix, 10, 32)
	return err == nil && chunkNo >= 0 && strconv.FormatInt(chunkNo, 10) == suffix
}

func filterBotCitations(tx *gorm.DB, citations []model.BotCitation) ([]model.BotCitation, error) {
	postIDs := make([]int64, 0, len(citations))
	seenIDs := make(map[int64]struct{}, len(citations))
	for _, citation := range citations {
		if citation.PostID > 0 {
			if _, exists := seenIDs[citation.PostID]; !exists {
				seenIDs[citation.PostID] = struct{}{}
				postIDs = append(postIDs, citation.PostID)
			}
		}
	}
	var visibleIDs []int64
	if len(postIDs) > 0 {
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Model(&model.Post{}).
			Where("id IN ? AND status = ? AND is_deleted = 0", postIDs, model.StatusApproved).
			Pluck("id", &visibleIDs).Error; err != nil {
			return nil, err
		}
	}
	visible := make(map[int64]struct{}, len(visibleIDs))
	for _, id := range visibleIDs {
		visible[id] = struct{}{}
	}
	filtered := make([]model.BotCitation, 0, len(citations))
	seenChunks := make(map[string]struct{}, len(citations))
	for _, citation := range citations {
		if _, ok := visible[citation.PostID]; !ok || !validChunkID(citation.PostID, citation.ChunkID) {
			continue
		}
		if _, duplicate := seenChunks[citation.ChunkID]; duplicate {
			continue
		}
		seenChunks[citation.ChunkID] = struct{}{}
		filtered = append(filtered, citation)
	}
	return filtered, nil
}

func (r *ChatRepo) CreateBotReply(
	ctx context.Context,
	sourceMessageID, conversationID int64,
	content string,
	citations []model.BotCitation,
) (*model.Message, bool, error) {
	var result *model.Message
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var source model.Message
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&source, sourceMessageID).Error; err != nil {
			return err
		}
		if source.ConversationID != conversationID {
			return ErrInvalidBotConversation
		}

		var members []model.ConversationMember
		if err := tx.Where("conversation_id = ?", conversationID).Preload("User").Find(&members).Error; err != nil {
			return err
		}
		if len(members) != 2 {
			return ErrInvalidBotConversation
		}
		var bot *model.User
		sourceIsActiveMember := false
		for _, member := range members {
			if member.User == nil || member.User.Status != model.UserStatusActive {
				return ErrInvalidBotConversation
			}
			if member.User.IsBot != 0 && member.User.Username == model.ShareOBotUsername {
				if bot != nil {
					return ErrInvalidBotConversation
				}
				copy := *member.User
				bot = &copy
			} else if member.UserID == source.SenderID && member.User.IsBot == 0 {
				sourceIsActiveMember = true
			}
		}
		if bot == nil || source.SenderID == bot.ID || !sourceIsActiveMember {
			return ErrInvalidBotConversation
		}

		var existing model.BotReply
		err := tx.Where("source_message_id = ?", sourceMessageID).First(&existing).Error
		if err == nil {
			var message model.Message
			if err := tx.Preload("Sender").First(&message, existing.ReplyMessageID).Error; err != nil {
				return err
			}
			result = &message
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		filtered, err := filterBotCitations(tx, citations)
		if err != nil {
			return err
		}
		metaBytes, err := json.Marshal(model.MessageMeta{
			SourceMessageID: sourceMessageID,
			Citations:       filtered,
		})
		if err != nil {
			return err
		}
		meta := string(metaBytes)
		message := &model.Message{
			ConversationID: conversationID,
			SenderID:       bot.ID,
			Content:        content,
			Meta:           &meta,
			Sender:         bot,
		}
		if err := tx.Create(message).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Conversation{}).Where("id = ?", conversationID).
			Update("updated_at", time.Now()).Error; err != nil {
			return err
		}
		memberUpdate := tx.Model(&model.ConversationMember{}).
			Where("conversation_id = ? AND user_id = ?", conversationID, bot.ID).
			Update("last_read_message_id", gorm.Expr("GREATEST(last_read_message_id, ?)", message.ID))
		if memberUpdate.Error != nil {
			return memberUpdate.Error
		}
		if memberUpdate.RowsAffected == 0 {
			return ErrInvalidBotConversation
		}
		if err := tx.Create(&model.BotReply{
			SourceMessageID: sourceMessageID,
			ReplyMessageID:  message.ID,
		}).Error; err != nil {
			return err
		}
		result = message
		created = true
		return nil
	})
	return result, created, err
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
	err := r.db.WithContext(ctx).Select("id", "username", "avatar_url", "is_bot").
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
