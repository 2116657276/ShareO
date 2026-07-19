package service

import (
	"context"
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/repository"
	"github.com/zhoujianlin/ShareO/internal/ws"
)

// ChatService handles IM business logic: conversations, messages, read tracking.
type ChatService struct {
	chatRepo *repository.ChatRepo
	userRepo *repository.UserRepo
	hub      *ws.Hub
}

// NewChatService creates a ChatService with injected dependencies.
func NewChatService(chatRepo *repository.ChatRepo, userRepo *repository.UserRepo, hub *ws.Hub) *ChatService {
	return &ChatService{chatRepo: chatRepo, userRepo: userRepo, hub: hub}
}

// --- Conversations ---

// EnsureDM returns the DM conversation between two users, creating it if necessary.
func (s *ChatService) EnsureDM(ctx context.Context, user1ID, user2ID int64) (*model.Conversation, error) {
	if user1ID == user2ID {
		return nil, errors.New("不能和自己创建私聊")
	}
	return s.chatRepo.EnsureDM(ctx, user1ID, user2ID)
}

// CreateGroup creates a new group conversation.
func (s *ChatService) CreateGroup(ctx context.Context, ownerID int64, title string, memberIDs []int64) (*model.Conversation, error) {
	if title == "" {
		return nil, errors.New("群组名称不能为空")
	}
	if len(memberIDs) == 0 {
		return nil, errors.New("至少需要邀请一位成员")
	}
	return s.chatRepo.CreateGroup(ctx, ownerID, title, memberIDs)
}

// JoinGroup adds a user to a group conversation.
func (s *ChatService) JoinGroup(ctx context.Context, convID, userID int64) error {
	conv, err := s.chatRepo.GetConversation(ctx, convID)
	if err != nil {
		return err
	}
	if conv.Type != model.ConvTypeGroup {
		return errors.New("只能加入群组")
	}
	return s.chatRepo.AddMember(ctx, convID, userID)
}

// LeaveGroup removes a user from a group conversation.
// The owner cannot leave — they must transfer ownership or dissolve the group first.
func (s *ChatService) LeaveGroup(ctx context.Context, convID, userID int64) error {
	conv, err := s.chatRepo.GetConversation(ctx, convID)
	if err != nil {
		return err
	}
	if conv.Type != model.ConvTypeGroup {
		return errors.New("私聊不能退出")
	}
	if conv.OwnerID == userID {
		return errors.New("群主不能退出，请先转让群主或解散群组")
	}
	return s.chatRepo.RemoveMember(ctx, convID, userID)
}

// --- Messages ---

// WsMessage is the JSON payload sent over WebSocket for a new message.
type WsMessage struct {
	Type string        `json:"type"`
	Data WsMessageData `json:"data"`
}

type WsMessageData struct {
	Message *model.Message `json:"message"`
}

// SendMessage validates membership, saves the message, and pushes to online members.
func (s *ChatService) SendMessage(ctx context.Context, senderID, convID int64, content string) (*model.Message, error) {
	if content == "" {
		return nil, errors.New("消息内容不能为空")
	}
	if !s.chatRepo.IsMember(ctx, convID, senderID) {
		return nil, errors.New("不是该会话的成员")
	}

	msg := &model.Message{
		ConversationID: convID,
		SenderID:       senderID,
		Content:        content,
	}
	if err := s.chatRepo.CreateMessage(ctx, msg); err != nil {
		return nil, err
	}

	// Push to online members of the conversation
	members, err := s.chatRepo.GetMembers(ctx, convID)
	if err != nil {
		log.Printf("ChatService.SendMessage: failed to get members for push: %v", err)
		return msg, nil // message is saved, push failure is non-fatal
	}

	memberIDs := make([]int64, len(members))
	for i, m := range members {
		memberIDs[i] = m.UserID
	}

	// Update the conversation's updated_at via a simple touch
	// (We don't need explicit update — the message creation itself signals activity)

	s.hub.SendToUsers(memberIDs, WsMessage{
		Type: "new_message",
		Data: WsMessageData{Message: msg},
	})

	return msg, nil
}

// GetMessages returns message history using cursor-based pagination.
func (s *ChatService) GetMessages(ctx context.Context, convID, userID int64, beforeID int64, limit int) ([]model.Message, error) {
	if !s.chatRepo.IsMember(ctx, convID, userID) {
		return nil, errors.New("不是该会话的成员")
	}
	return s.chatRepo.GetMessages(ctx, convID, beforeID, limit)
}

// --- Read tracking ---

// MarkRead marks messages up to messageID as read for the user in this conversation.
func (s *ChatService) MarkRead(ctx context.Context, convID, userID, messageID int64) error {
	if !s.chatRepo.IsMember(ctx, convID, userID) {
		return errors.New("不是该会话的成员")
	}
	return s.chatRepo.UpdateReadMarker(ctx, convID, userID, messageID)
}

// UnreadCount returns the total unread message count across all conversations.
func (s *ChatService) UnreadCount(ctx context.Context, userID int64) (int64, error) {
	return s.chatRepo.GetUnreadCount(ctx, userID)
}

// --- List ---

// ConversationWithMeta enriches a conversation with last message, members, and unread count.
type ConversationWithMeta struct {
	model.Conversation
	LastMessage *model.Message             `json:"last_message,omitempty"`
	Members     []model.ConversationMember `json:"members,omitempty"`
	UnreadCount int64                      `json:"unread_count"`
}

// ListConversations returns all conversations for a user with metadata.
func (s *ChatService) ListConversations(ctx context.Context, userID int64) ([]ConversationWithMeta, error) {
	convs, err := s.chatRepo.ListConversations(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]ConversationWithMeta, 0, len(convs))
	for _, conv := range convs {
		// Get last message
		lastMsg, _ := s.chatRepo.GetLastMessage(ctx, conv.ID)

		// Get members
		members, _ := s.chatRepo.GetMembers(ctx, conv.ID)

		// Get unread count for this user
		lastReadID, _ := s.chatRepo.GetLastReadMessageID(ctx, conv.ID, userID)
		var unread int64
		if lastMsg != nil && lastMsg.ID > lastReadID {
			// Count messages > last_read up to last message
			msgs, _ := s.chatRepo.GetMessages(ctx, conv.ID, 0, 1)
			if len(msgs) > 0 && msgs[0].ID > lastReadID {
				// Approximate: count all messages since last_read (simplified for v1)
				unread = 1 // at least one unread if latest > lastRead
			}
		}

		// For DM, use the other user's info for display
		if conv.Type == model.ConvTypeDM && conv.Title == "" {
			for _, m := range members {
				if m.UserID != userID && m.User != nil {
					conv.Title = m.User.Username
					break
				}
			}
		}

		result = append(result, ConversationWithMeta{
			Conversation: conv,
			LastMessage:  lastMsg,
			Members:      members,
			UnreadCount:  unread,
		})
	}

	return result, nil
}

// --- Online status ---

// SetUserOnline records a user as online in Redis with a 60s TTL.
// Called from the WS upgrade handler; heartbeat is refreshed via WS pings.
func (s *ChatService) SetUserOnline(ctx context.Context, userID int64) error {
	return repository.RDB.Set(ctx, onlineKey(userID), time.Now().Unix(), 60*time.Second).Err()
}

// IsUserOnline checks if a user has an active WebSocket connection.
func (s *ChatService) IsUserOnline(ctx context.Context, userID int64) bool {
	// First check the in-memory Hub (fast path)
	if s.hub.IsOnline(userID) {
		return true
	}
	// Fallback to Redis (catches edge cases where hub state is stale)
	val, err := repository.RDB.Get(ctx, onlineKey(userID)).Result()
	return err == nil && val != ""
}

func onlineKey(userID int64) string {
	return "ws:online:" + strconv.FormatInt(userID, 10)
}
