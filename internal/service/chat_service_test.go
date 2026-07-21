package service

import (
	"context"
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
	createdGroup  []int64
	removedUser   int64
	createdMsg    *model.Message
	markReadErr   error
	requestedPage [3]int64
}

func (f *fakeChatRepo) ValidateActiveUsers(_ context.Context, ids []int64) error {
	f.validatedIDs = append([]int64(nil), ids...)
	return nil
}
func (f *fakeChatRepo) EnsureDM(_ context.Context, a, _ int64) (*model.Conversation, error) {
	return &model.Conversation{ID: a, Type: model.ConvTypeDM}, nil
}
func (f *fakeChatRepo) CreateGroup(_ context.Context, owner int64, title string, ids []int64) (*model.Conversation, error) {
	f.createdGroup = append([]int64(nil), ids...)
	return &model.Conversation{ID: 12, Type: model.ConvTypeGroup, OwnerID: &owner, Title: title}, nil
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
	return []model.ConversationMember{{UserID: 1}, {UserID: 2}}, nil
}
func (f *fakeChatRepo) AddMembers(context.Context, int64, []int64, int) (bool, error) {
	return true, nil
}
func (f *fakeChatRepo) RemoveMember(_ context.Context, _ int64, userID int64) error {
	f.removedUser = userID
	return nil
}
func (f *fakeChatRepo) DissolveGroup(context.Context, int64) error { return nil }
func (f *fakeChatRepo) CreateMessage(_ context.Context, msg *model.Message) error {
	msg.ID = 99
	f.createdMsg = msg
	return nil
}
func (f *fakeChatRepo) GetMessages(_ context.Context, _ int64, before, after int64, limit int) ([]model.Message, error) {
	f.requestedPage = [3]int64{before, after, int64(limit)}
	return []model.Message{{ID: 3}}, nil
}
func (f *fakeChatRepo) GetLastMessage(context.Context, int64) (*model.Message, error) {
	return nil, nil
}
func (f *fakeChatRepo) GetRecentMessages(context.Context, int64, int) ([]model.Message, error) {
	return nil, nil
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

func TestCreateGroupNormalizesMembersAndTitle(t *testing.T) {
	repo := &fakeChatRepo{}
	svc, _ := newChatServiceForTest(repo)
	conv, err := svc.CreateGroup(context.Background(), 1, "  Team  ", []int64{2, 2, 1, 3})
	if err != nil {
		t.Fatal(err)
	}
	if conv.Title != "Team" || len(repo.createdGroup) != 2 || repo.createdGroup[0] != 2 || repo.createdGroup[1] != 3 {
		t.Fatalf("unexpected normalized group: %#v members=%v", conv, repo.createdGroup)
	}
}

func TestGroupPermissionRules(t *testing.T) {
	ownerID := int64(1)
	repo := &fakeChatRepo{conversation: &model.Conversation{ID: 5, Type: model.ConvTypeGroup, OwnerID: &ownerID}, isMember: true}
	svc, _ := newChatServiceForTest(repo)
	if err := svc.InviteMembers(context.Background(), 5, 2, []int64{3}); !errors.Is(err, ErrChatForbidden) {
		t.Fatalf("non-owner invite error = %v", err)
	}
	if err := svc.LeaveGroup(context.Background(), 5, 1); !errors.Is(err, ErrChatConflict) {
		t.Fatalf("owner leave error = %v", err)
	}
	if err := svc.LeaveGroup(context.Background(), 5, 2); err != nil || repo.removedUser != 2 {
		t.Fatalf("member leave error = %v, removed=%d", err, repo.removedUser)
	}
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
