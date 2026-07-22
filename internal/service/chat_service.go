package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/repository"
	"gorm.io/gorm"
)

const (
	MaxMessageLength = 2000
	presenceTTL      = 60 * time.Second
)

var (
	ErrChatInvalid   = errors.New("chat invalid input")
	ErrChatForbidden = errors.New("chat forbidden")
	ErrChatNotFound  = errors.New("chat not found")
	ErrChatConflict  = errors.New("chat conflict")
)

type ChatRepository interface {
	ValidateActiveUsers(context.Context, []int64) error
	EnsureDM(context.Context, int64, int64) (*model.Conversation, error)
	GetConversation(context.Context, int64) (*model.Conversation, error)
	ListConversations(context.Context, int64) ([]model.Conversation, error)
	IsMember(context.Context, int64, int64) (bool, error)
	GetMembers(context.Context, int64) ([]model.ConversationMember, error)
	CreateMessage(context.Context, *model.Message) error
	GetMessageByID(context.Context, int64) (*model.Message, error)
	CreateBotReply(context.Context, int64, int64, string, []model.BotCitation) (*model.Message, bool, error)
	VisiblePostIDs(context.Context, []int64) (map[int64]struct{}, error)
	GetMessages(context.Context, int64, int64, int64, int) ([]model.Message, error)
	GetLastMessage(context.Context, int64) (*model.Message, error)
	GetRecentMessages(context.Context, int64, int) ([]model.Message, error)
	UpdateReadMarker(context.Context, int64, int64, int64) error
	GetUnreadCount(context.Context, int64) (int64, error)
	GetConversationUnreadCount(context.Context, int64, int64) (int64, error)
	SearchActiveUsers(context.Context, string, int64, int) ([]model.User, error)
}

type PresenceStore interface {
	SetOnline(context.Context, int64, time.Duration) error
	SetOffline(context.Context, int64) error
	IsOnline(context.Context, int64) (bool, error)
}

type MessageHub interface {
	SendToUsers([]int64, any)
	IsOnline(int64) bool
	DisconnectUser(int64)
}

type ChatService struct {
	repo     ChatRepository
	presence PresenceStore
	hub      MessageHub
	aiBridge *AIBridge
}

func NewChatService(repo ChatRepository, presence PresenceStore, hub MessageHub, bridges ...*AIBridge) *ChatService {
	service := &ChatService{repo: repo, presence: presence, hub: hub}
	if len(bridges) > 0 {
		service.aiBridge = bridges[0]
	}
	return service
}

func chatError(kind error, message string) error { return fmt.Errorf("%w: %s", kind, message) }

func (s *ChatService) EnsureDM(ctx context.Context, user1ID, user2ID int64) (*model.Conversation, error) {
	if user2ID <= 0 || user1ID == user2ID {
		return nil, chatError(ErrChatInvalid, "不能和自己或无效用户创建私聊")
	}
	if err := s.repo.ValidateActiveUsers(ctx, []int64{user1ID, user2ID}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, chatError(ErrChatNotFound, "用户不存在或不可用")
		}
		return nil, err
	}
	return s.repo.EnsureDM(ctx, user1ID, user2ID)
}

type WsMessage struct {
	Type string        `json:"type"`
	Data WsMessageData `json:"data"`
}

type WsMessageData struct {
	Message *model.Message `json:"message"`
}

func (s *ChatService) SendMessage(ctx context.Context, senderID, convID int64, content string) (*model.Message, error) {
	content = strings.TrimSpace(content)
	if content == "" || len([]rune(content)) > MaxMessageLength {
		return nil, chatError(ErrChatInvalid, "消息长度必须为 1-2000 个字符")
	}
	if err := s.requireMember(ctx, convID, senderID); err != nil {
		return nil, err
	}
	message := &model.Message{ConversationID: convID, SenderID: senderID, Content: content}
	if err := s.repo.CreateMessage(ctx, message); err != nil {
		return nil, err
	}
	members, err := s.repo.GetMembers(ctx, convID)
	if err != nil {
		// The message is already committed. Clients recover it through after_id,
		// so report success while retaining an operational signal.
		slog.Warn("message committed but websocket fanout members could not be loaded",
			"conv_id", convID, "message_id", message.ID, "err", err)
		return message, nil
	}
	memberIDs := make([]int64, 0, len(members))
	for _, member := range members {
		memberIDs = append(memberIDs, member.UserID)
	}
	s.hub.SendToUsers(memberIDs, WsMessage{Type: "new_message", Data: WsMessageData{Message: message}})
	senderIsBot := false
	for _, member := range members {
		if member.UserID == senderID {
			senderIsBot = member.User != nil && member.User.IsBot != 0
			break
		}
	}
	if senderIsBot {
		return message, nil
	}
	if len(members) == 2 {
		for _, member := range members {
			if member.UserID != senderID && member.User != nil && member.User.IsBot != 0 && member.User.Username == model.ShareOBotUsername {
				s.aiBridge.PublishBotTask(convID, message.ID)
				break
			}
		}
	}
	return message, nil
}

func (s *ChatService) GetMessages(ctx context.Context, convID, userID, beforeID, afterID int64, limit int) ([]model.Message, error) {
	if beforeID > 0 && afterID > 0 {
		return nil, chatError(ErrChatInvalid, "before_id 与 after_id 不能同时使用")
	}
	if err := s.requireMember(ctx, convID, userID); err != nil {
		return nil, err
	}
	messages, err := s.repo.GetMessages(ctx, convID, beforeID, afterID, limit)
	if err != nil {
		return nil, err
	}
	if err := s.sanitizeMessageCitations(ctx, messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func (s *ChatService) MarkRead(ctx context.Context, convID, userID, messageID int64) error {
	if messageID <= 0 {
		return chatError(ErrChatInvalid, "消息 ID 无效")
	}
	if err := s.requireMember(ctx, convID, userID); err != nil {
		return err
	}
	if err := s.repo.UpdateReadMarker(ctx, convID, userID, messageID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return chatError(ErrChatNotFound, "消息不存在")
		}
		return err
	}
	return nil
}

func (s *ChatService) UnreadCount(ctx context.Context, userID int64) (int64, error) {
	return s.repo.GetUnreadCount(ctx, userID)
}

type ConversationWithMeta struct {
	model.Conversation
	LastMessage *model.Message             `json:"last_message,omitempty"`
	Members     []model.ConversationMember `json:"members,omitempty"`
	UnreadCount int64                      `json:"unread_count"`
	Online      bool                       `json:"online"`
}

func (s *ChatService) ListConversations(ctx context.Context, userID int64) ([]ConversationWithMeta, error) {
	conversations, err := s.repo.ListConversations(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]ConversationWithMeta, 0, len(conversations))
	for _, conv := range conversations {
		lastMessage, err := s.repo.GetLastMessage(ctx, conv.ID)
		if err != nil {
			return nil, err
		}
		if lastMessage != nil {
			messages := []model.Message{*lastMessage}
			if err := s.sanitizeMessageCitations(ctx, messages); err != nil {
				return nil, err
			}
			lastMessage = &messages[0]
		}
		members, err := s.repo.GetMembers(ctx, conv.ID)
		if err != nil {
			return nil, err
		}
		unread, err := s.repo.GetConversationUnreadCount(ctx, conv.ID, userID)
		if err != nil {
			return nil, err
		}
		online := false
		for _, member := range members {
			if member.UserID != userID && member.User != nil {
				conv.Title = member.User.Username
				online = s.IsUserOnline(ctx, member.UserID)
				break
			}
		}
		result = append(result, ConversationWithMeta{
			Conversation: conv, LastMessage: lastMessage, Members: members,
			UnreadCount: unread, Online: online,
		})
	}
	return result, nil
}

func (s *ChatService) SearchUsers(ctx context.Context, query string, callerID int64, limit int) ([]model.User, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, chatError(ErrChatInvalid, "搜索关键词不能为空")
	}
	if limit <= 0 || limit > 20 {
		limit = 20
	}
	return s.repo.SearchActiveUsers(ctx, query, callerID, limit)
}

func (s *ChatService) SetUserOnline(ctx context.Context, userID int64) error {
	return s.presence.SetOnline(ctx, userID, presenceTTL)
}

func (s *ChatService) SetUserOffline(ctx context.Context, userID int64) error {
	if s.hub.IsOnline(userID) {
		return nil
	}
	return s.presence.SetOffline(ctx, userID)
}

func (s *ChatService) IsUserOnline(ctx context.Context, userID int64) bool {
	if s.hub.IsOnline(userID) {
		return true
	}
	online, err := s.presence.IsOnline(ctx, userID)
	return err == nil && online
}

func (s *ChatService) DisconnectUser(userID int64) { s.hub.DisconnectUser(userID) }

func (s *ChatService) ValidateUser(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return chatError(ErrChatForbidden, "账号不可用")
	}
	if err := s.repo.ValidateActiveUsers(ctx, []int64{userID}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return chatError(ErrChatForbidden, "账号不可用")
		}
		return err
	}
	return nil
}

func (s *ChatService) conversation(ctx context.Context, convID int64) (*model.Conversation, error) {
	if convID <= 0 {
		return nil, chatError(ErrChatInvalid, "会话 ID 无效")
	}
	conv, err := s.repo.GetConversation(ctx, convID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, chatError(ErrChatNotFound, "会话不存在")
	}
	return conv, err
}

func (s *ChatService) requireMember(ctx context.Context, convID, userID int64) error {
	if _, err := s.conversation(ctx, convID); err != nil {
		return err
	}
	member, err := s.repo.IsMember(ctx, convID, userID)
	if err != nil {
		return err
	}
	if !member {
		return chatError(ErrChatForbidden, "不是该会话的成员")
	}
	return nil
}

type BotTask struct {
	Message      *model.Message      `json:"message"`
	History      []model.Message     `json:"history"`
	Bot          *model.User         `json:"bot"`
	Conversation *model.Conversation `json:"conversation"`
}

func (s *ChatService) GetBotTask(ctx context.Context, messageID int64) (*BotTask, error) {
	if messageID <= 0 {
		return nil, chatError(ErrChatInvalid, "消息 ID 无效")
	}
	message, err := s.repo.GetMessageByID(ctx, messageID)
	if err != nil {
		return nil, err
	}
	if message == nil {
		return nil, chatError(ErrChatNotFound, "消息不存在")
	}
	conversation, err := s.conversation(ctx, message.ConversationID)
	if err != nil {
		return nil, err
	}
	members, err := s.repo.GetMembers(ctx, conversation.ID)
	if err != nil {
		return nil, err
	}
	if len(members) != 2 {
		return nil, chatError(ErrChatForbidden, "不是有效的 Bot 私聊")
	}
	var bot *model.User
	senderIsMember := false
	for _, member := range members {
		if member.User == nil || member.User.Status != model.UserStatusActive {
			return nil, chatError(ErrChatForbidden, "会话成员不可用")
		}
		if member.User.IsBot != 0 && member.User.Username == model.ShareOBotUsername {
			if bot != nil {
				return nil, chatError(ErrChatForbidden, "不是有效的 Bot 私聊")
			}
			copy := *member.User
			bot = &copy
		} else if member.UserID == message.SenderID && member.User.IsBot == 0 {
			senderIsMember = true
		}
	}
	if bot == nil || bot.ID == message.SenderID || !senderIsMember {
		return nil, chatError(ErrChatForbidden, "不是有效的 Bot 私聊")
	}
	history, err := s.repo.GetRecentMessages(ctx, conversation.ID, 20)
	if err != nil {
		return nil, err
	}
	for left, right := 0, len(history)-1; left < right; left, right = left+1, right-1 {
		history[left], history[right] = history[right], history[left]
	}
	return &BotTask{Message: message, History: history, Bot: bot, Conversation: conversation}, nil
}

func (s *ChatService) ReplyAsBot(
	ctx context.Context,
	sourceMessageID, conversationID int64,
	content string,
	citations []model.BotCitation,
) (*model.Message, error) {
	content = strings.TrimSpace(content)
	if sourceMessageID <= 0 || conversationID <= 0 || content == "" || len([]rune(content)) > MaxMessageLength {
		return nil, chatError(ErrChatInvalid, "Bot 回复参数无效")
	}
	if len(citations) > 5 {
		return nil, chatError(ErrChatInvalid, "引用数量不能超过 5")
	}
	message, created, err := s.repo.CreateBotReply(
		ctx, sourceMessageID, conversationID, content, citations,
	)
	if errors.Is(err, repository.ErrInvalidBotConversation) || errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, chatError(ErrChatForbidden, "不是有效的 Bot 私聊")
	}
	if err != nil {
		return nil, err
	}
	messages := []model.Message{*message}
	if err := s.sanitizeMessageCitations(ctx, messages); err != nil {
		return nil, err
	}
	message = &messages[0]
	if !created {
		return message, nil
	}
	members, err := s.repo.GetMembers(ctx, conversationID)
	if err != nil {
		slog.Warn("bot reply committed but websocket members could not be loaded",
			"conversation_id", conversationID, "message_id", message.ID, "err", err)
		return message, nil
	}
	memberIDs := make([]int64, 0, len(members))
	for _, member := range members {
		memberIDs = append(memberIDs, member.UserID)
	}
	s.hub.SendToUsers(memberIDs, WsMessage{Type: "new_message", Data: WsMessageData{Message: message}})
	return message, nil
}

func (s *ChatService) sanitizeMessageCitations(ctx context.Context, messages []model.Message) error {
	postIDs := make([]int64, 0)
	metas := make([]*model.MessageMeta, len(messages))
	for index := range messages {
		if messages[index].Meta == nil {
			continue
		}
		var meta model.MessageMeta
		if err := json.Unmarshal([]byte(*messages[index].Meta), &meta); err != nil {
			messages[index].Meta = nil
			continue
		}
		metas[index] = &meta
		for _, citation := range meta.Citations {
			postIDs = append(postIDs, citation.PostID)
		}
	}
	visible, err := s.repo.VisiblePostIDs(ctx, postIDs)
	if err != nil {
		return err
	}
	for index, meta := range metas {
		if meta == nil {
			continue
		}
		filtered := meta.Citations[:0]
		for _, citation := range meta.Citations {
			if _, ok := visible[citation.PostID]; ok {
				filtered = append(filtered, citation)
			}
		}
		meta.Citations = filtered
		encoded, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		value := string(encoded)
		messages[index].Meta = &value
	}
	return nil
}
