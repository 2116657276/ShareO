package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm"
)

type fakeChatRepo struct {
	conversation  *model.Conversation
	isMember      bool
	unread        int64
	validatedIDs  []int64
	createdMsg    *model.Message
	markReadErr   error
	requestedPage [3]int64
	members       []model.ConversationMember
	recent        []model.Message
	visible       map[int64]struct{}
	messages      []model.Message
	messageByID   *model.Message
	botReply      *model.Message
	botCreated    bool
}

func (f *fakeChatRepo) ValidateActiveUsers(_ context.Context, ids []int64) error {
	f.validatedIDs = append([]int64(nil), ids...)
	return nil
}
func (f *fakeChatRepo) EnsureDM(_ context.Context, a, _ int64) (*model.Conversation, error) {
	return &model.Conversation{ID: a}, nil
}
func (f *fakeChatRepo) GetConversation(context.Context, int64) (*model.Conversation, error) {
	if f.conversation == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return f.conversation, nil
}
func (f *fakeChatRepo) ListConversations(context.Context, int64) ([]model.Conversation, error) {
	return nil, nil
}
func (f *fakeChatRepo) IsMember(context.Context, int64, int64) (bool, error) { return f.isMember, nil }
func (f *fakeChatRepo) GetMembers(context.Context, int64) ([]model.ConversationMember, error) {
	if f.members != nil {
		return f.members, nil
	}
	return []model.ConversationMember{{UserID: 1}, {UserID: 2}}, nil
}
func (f *fakeChatRepo) CreateMessage(_ context.Context, msg *model.Message) error {
	msg.ID = 99
	f.createdMsg = msg
	return nil
}
func (f *fakeChatRepo) GetMessageByID(context.Context, int64) (*model.Message, error) {
	return f.messageByID, nil
}
func (f *fakeChatRepo) CreateBotReply(_ context.Context, _, _ int64, content string, _ []model.BotCitation) (*model.Message, bool, error) {
	if f.botReply != nil {
		return f.botReply, f.botCreated, nil
	}
	return &model.Message{ID: 100, Content: content}, true, nil
}
func (f *fakeChatRepo) VisiblePostIDs(context.Context, []int64) (map[int64]struct{}, error) {
	return f.visible, nil
}
func (f *fakeChatRepo) GetMessages(_ context.Context, _ int64, before, after int64, limit int) ([]model.Message, error) {
	f.requestedPage = [3]int64{before, after, int64(limit)}
	if f.messages != nil {
		return f.messages, nil
	}
	return []model.Message{{ID: 3}}, nil
}
func (f *fakeChatRepo) GetLastMessage(context.Context, int64) (*model.Message, error) {
	return nil, nil
}
func (f *fakeChatRepo) GetRecentMessages(context.Context, int64, int) ([]model.Message, error) {
	return f.recent, nil
}
func (f *fakeChatRepo) UpdateReadMarker(context.Context, int64, int64, int64) error {
	return f.markReadErr
}
func (f *fakeChatRepo) GetUnreadCount(context.Context, int64) (int64, error) { return f.unread, nil }
func (f *fakeChatRepo) GetConversationUnreadCount(context.Context, int64, int64) (int64, error) {
	return f.unread, nil
}
func (f *fakeChatRepo) SearchActiveUsers(context.Context, string, int64, int) ([]model.User, error) {
	return nil, nil
}

type fakePresence struct{}

func (fakePresence) SetOnline(context.Context, int64, time.Duration) error { return nil }
func (fakePresence) SetOffline(context.Context, int64) error               { return nil }
func (fakePresence) IsOnline(context.Context, int64) (bool, error)         { return false, nil }

type fakeHub struct{ sent int }

func (h *fakeHub) SendToUsers(_ []int64, _ any) { h.sent++ }
func (*fakeHub) IsOnline(int64) bool            { return false }
func (*fakeHub) DisconnectUser(int64)           {}

func newChatServiceForTest(repo *fakeChatRepo) (*ChatService, *fakeHub) {
	hub := &fakeHub{}
	return NewChatService(repo, fakePresence{}, hub), hub
}

func TestMessageLimitsAndBroadcast(t *testing.T) {
	repo := &fakeChatRepo{conversation: &model.Conversation{ID: 5}, isMember: true}
	svc, hub := newChatServiceForTest(repo)
	if _, err := svc.SendMessage(context.Background(), 2, 5, strings.Repeat("界", MaxMessageLength+1)); !errors.Is(err, ErrChatInvalid) {
		t.Fatalf("oversized message error = %v", err)
	}
	msg, err := svc.SendMessage(context.Background(), 2, 5, "  hello  ")
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "hello" || msg.ID != 99 || hub.sent != 1 {
		t.Fatalf("message=%#v broadcasts=%d", msg, hub.sent)
	}
}

func TestPaginationConflictAndUnreadDelegation(t *testing.T) {
	repo := &fakeChatRepo{conversation: &model.Conversation{ID: 5}, isMember: true, unread: 7}
	svc, _ := newChatServiceForTest(repo)
	if _, err := svc.GetMessages(context.Background(), 5, 2, 3, 4, 30); !errors.Is(err, ErrChatInvalid) {
		t.Fatalf("pagination error = %v", err)
	}
	msgs, err := svc.GetMessages(context.Background(), 5, 2, 0, 4, 20)
	if err != nil || len(msgs) != 1 || repo.requestedPage != [3]int64{0, 4, 20} {
		t.Fatalf("messages=%v request=%v err=%v", msgs, repo.requestedPage, err)
	}
	unread, err := svc.UnreadCount(context.Background(), 2)
	if err != nil || unread != 7 {
		t.Fatalf("unread=%d err=%v", unread, err)
	}
}

func TestMarkReadMapsForeignMessageToNotFound(t *testing.T) {
	repo := &fakeChatRepo{conversation: &model.Conversation{ID: 5}, isMember: true, markReadErr: gorm.ErrRecordNotFound}
	svc, _ := newChatServiceForTest(repo)
	if err := svc.MarkRead(context.Background(), 5, 2, 100); !errors.Is(err, ErrChatNotFound) {
		t.Fatalf("mark read error = %v", err)
	}
}

func TestBotTaskRequiresBotDMAndReturnsChronologicalHistory(t *testing.T) {
	user := &model.User{ID: 1, Status: model.UserStatusActive}
	bot := &model.User{ID: 2, Username: model.ShareOBotUsername, Status: model.UserStatusActive, IsBot: 1}
	repo := &fakeChatRepo{
		conversation: &model.Conversation{ID: 5},
		messageByID:  &model.Message{ID: 9, ConversationID: 5, SenderID: user.ID},
		members: []model.ConversationMember{
			{UserID: user.ID, User: user}, {UserID: bot.ID, User: bot},
		},
		recent: []model.Message{{ID: 9}, {ID: 8}},
	}
	svc, _ := newChatServiceForTest(repo)
	task, err := svc.GetBotTask(context.Background(), 9)
	if err != nil || task.Bot.ID != bot.ID || task.History[0].ID != 8 || task.History[1].ID != 9 {
		t.Fatalf("task=%+v err=%v", task, err)
	}

	ordinary := &model.User{ID: 3, Status: model.UserStatusActive}
	repo.members[1] = model.ConversationMember{UserID: ordinary.ID, User: ordinary}
	if _, err := svc.GetBotTask(context.Background(), 9); !errors.Is(err, ErrChatForbidden) {
		t.Fatalf("ordinary DM bot task error=%v", err)
	}
}

func TestMessageHistoryDropsCitationsForInvisiblePosts(t *testing.T) {
	metaBytes, _ := json.Marshal(model.MessageMeta{Citations: []model.BotCitation{
		{PostID: 10, ChunkID: "10:0"}, {PostID: 11, ChunkID: "11:0"},
	}})
	meta := string(metaBytes)
	repo := &fakeChatRepo{
		conversation: &model.Conversation{ID: 5},
		isMember:     true,
		visible:      map[int64]struct{}{10: {}},
		messages:     []model.Message{{ID: 9, Meta: &meta}},
	}
	svc, _ := newChatServiceForTest(repo)
	messages, err := svc.GetMessages(context.Background(), 5, 1, 0, 0, 30)
	if err != nil {
		t.Fatal(err)
	}
	var filtered model.MessageMeta
	if err := json.Unmarshal([]byte(*messages[0].Meta), &filtered); err != nil {
		t.Fatal(err)
	}
	if len(filtered.Citations) != 1 || filtered.Citations[0].PostID != 10 {
		t.Fatalf("citations=%v", filtered.Citations)
	}
}
