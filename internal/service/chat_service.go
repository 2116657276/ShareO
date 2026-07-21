package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm"
)

const (
	MaxMessageLength = 2000
	MaxGroupTitle    = 100
	MaxGroupMembers  = 50
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
	CreateGroup(context.Context, int64, string, []int64) (*model.Conversation, error)
	GetConversation(context.Context, int64) (*model.Conversation, error)
	ListConversations(context.Context, int64) ([]model.Conversation, error)
	IsMember(context.Context, int64, int64) (bool, error)
	GetMembers(context.Context, int64) ([]model.ConversationMember, error)
	AddMembers(context.Context, int64, []int64, int) (bool, error)
	RemoveMember(context.Context, int64, int64) error
	DissolveGroup(context.Context, int64) error
	CreateMessage(context.Context, *model.Message) error
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
}

func NewChatService(repo ChatRepository, presence PresenceStore, hub MessageHub) *ChatService {
	return &ChatService{repo: repo, presence: presence, hub: hub}
}

func chatError(kind error, message string) error { return fmt.Errorf("%w: %s", kind, message) }

func normalizeIDs(ids []int64, excluded int64) ([]int64, error) {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, chatError(ErrChatInvalid, "用户 ID 无效")
		}
		if id == excluded {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result, nil
}

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

func (s *ChatService) CreateGroup(ctx context.Context, ownerID int64, title string, memberIDs []int64) (*model.Conversation, error) {
	title = strings.TrimSpace(title)
	if title == "" || len([]rune(title)) > MaxGroupTitle {
		return nil, chatError(ErrChatInvalid, "群组名称长度必须为 1-100 个字符")
	}
	members, err := normalizeIDs(memberIDs, ownerID)
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, chatError(ErrChatInvalid, "至少需要邀请一位成员")
	}
	if len(members)+1 > MaxGroupMembers {
		return nil, chatError(ErrChatInvalid, "群组成员不能超过 50 人")
	}
	if err := s.repo.ValidateActiveUsers(ctx, append(members, ownerID)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, chatError(ErrChatNotFound, "邀请的用户不存在或不可用")
		}
		return nil, err
	}
	return s.repo.CreateGroup(ctx, ownerID, title, members)
}

func (s *ChatService) InviteMembers(ctx context.Context, convID, ownerID int64, userIDs []int64) error {
	conv, err := s.conversation(ctx, convID)
	if err != nil {
		return err
	}
	if conv.Type != model.ConvTypeGroup || !isOwner(conv, ownerID) {
		return chatError(ErrChatForbidden, "仅群主可以邀请成员")
	}
	ids, err := normalizeIDs(userIDs, ownerID)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return chatError(ErrChatInvalid, "至少选择一位成员")
	}
	current, err := s.repo.GetMembers(ctx, convID)
	if err != nil {
		return err
	}
	existing := make(map[int64]struct{}, len(current))
	for _, member := range current {
		existing[member.UserID] = struct{}{}
	}
	filtered := ids[:0]
	for _, id := range ids {
		if _, ok := existing[id]; !ok {
			filtered = append(filtered, id)
		}
	}
	if len(current)+len(filtered) > MaxGroupMembers {
		return chatError(ErrChatConflict, "群组成员不能超过 50 人")
	}
	if len(filtered) == 0 {
		return nil
	}
	if err := s.repo.ValidateActiveUsers(ctx, filtered); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return chatError(ErrChatNotFound, "邀请的用户不存在或不可用")
		}
		return err
	}
	withinLimit, err := s.repo.AddMembers(ctx, convID, filtered, MaxGroupMembers)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return chatError(ErrChatNotFound, "会话不存在")
		}
		return err
	}
	if !withinLimit {
		return chatError(ErrChatConflict, "群组成员不能超过 50 人")
	}
	return nil
}

func (s *ChatService) LeaveGroup(ctx context.Context, convID, userID int64) error {
	conv, err := s.conversation(ctx, convID)
	if err != nil {
		return err
	}
	if conv.Type != model.ConvTypeGroup {
		return chatError(ErrChatInvalid, "私聊不能退出")
	}
	if isOwner(conv, userID) {
		return chatError(ErrChatConflict, "群主不能退出，只能解散群组")
	}
	member, err := s.repo.IsMember(ctx, convID, userID)
	if err != nil {
		return err
	}
	if !member {
		return chatError(ErrChatForbidden, "不是该群组的成员")
	}
	return s.repo.RemoveMember(ctx, convID, userID)
}

func (s *ChatService) DissolveGroup(ctx context.Context, convID, userID int64) error {
	conv, err := s.conversation(ctx, convID)
	if err != nil {
		return err
	}
	if conv.Type != model.ConvTypeGroup || !isOwner(conv, userID) {
		return chatError(ErrChatForbidden, "仅群主可以解散群组")
	}
	if err := s.repo.DissolveGroup(ctx, convID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return chatError(ErrChatNotFound, "会话不存在")
		}
		return err
	}
	return nil
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
	return message, nil
}

func (s *ChatService) GetMessages(ctx context.Context, convID, userID, beforeID, afterID int64, limit int) ([]model.Message, error) {
	if beforeID > 0 && afterID > 0 {
		return nil, chatError(ErrChatInvalid, "before_id 与 after_id 不能同时使用")
	}
	if err := s.requireMember(ctx, convID, userID); err != nil {
		return nil, err
	}
	return s.repo.GetMessages(ctx, convID, beforeID, afterID, limit)
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
		members, err := s.repo.GetMembers(ctx, conv.ID)
		if err != nil {
			return nil, err
		}
		unread, err := s.repo.GetConversationUnreadCount(ctx, conv.ID, userID)
		if err != nil {
			return nil, err
		}
		online := false
		if conv.Type == model.ConvTypeDM {
			for _, member := range members {
				if member.UserID != userID && member.User != nil {
					conv.Title = member.User.Username
					online = s.IsUserOnline(ctx, member.UserID)
					break
				}
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

func isOwner(conv *model.Conversation, userID int64) bool {
	return conv.OwnerID != nil && *conv.OwnerID == userID
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
