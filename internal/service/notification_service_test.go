package service

import (
	"errors"
	"testing"

	"github.com/zhoujianlin/ShareO/internal/model"
)

// mockNotifRepo implements notifRepo for testing.
// It records created notifications and supports configurable errors.
type mockNotifRepo struct {
	notifs      []model.Notification
	err         error
	unreadCount int64

	// lastList captures the parameters passed to List()
	lastListUserID     int64
	lastListUnreadOnly bool
	lastListPage       int
	lastListPageSize   int

	// lastMarkRead captures parameters
	lastMarkReadID     int64
	lastMarkReadUserID int64

	// lastMarkAllRead captures parameters
	lastMarkAllReadUserID int64

	// lastUnreadCountUserID captures parameter
	lastUnreadCountUserID int64
}

func (m *mockNotifRepo) Create(notif *model.Notification) error {
	if m.err != nil {
		return m.err
	}
	m.notifs = append(m.notifs, *notif)
	return nil
}

func (m *mockNotifRepo) List(userID int64, unreadOnly bool, page, pageSize int) ([]model.Notification, int64, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	m.lastListUserID = userID
	m.lastListUnreadOnly = unreadOnly
	m.lastListPage = page
	m.lastListPageSize = pageSize
	return m.notifs, int64(len(m.notifs)), nil
}

func (m *mockNotifRepo) MarkRead(id, userID int64) error {
	m.lastMarkReadID = id
	m.lastMarkReadUserID = userID
	return m.err
}

func (m *mockNotifRepo) MarkAllRead(userID int64) error {
	m.lastMarkAllReadUserID = userID
	return m.err
}

func (m *mockNotifRepo) UnreadCount(userID int64) int64 {
	m.lastUnreadCountUserID = userID
	return m.unreadCount
}

func TestNotificationService_Send_SkipsSelf(t *testing.T) {
	mock := &mockNotifRepo{}
	svc := &NotificationService{repo: mock}

	// userID == actorID should be a no-op
	svc.Send(1, 1, model.NotifTypeLike, 42)

	if len(mock.notifs) != 0 {
		t.Errorf("expected 0 notifications for self-action, got %d", len(mock.notifs))
	}

	total, failed := svc.Stats()
	if total != 0 || failed != 0 {
		t.Errorf("Stats after skipped send = (%d, %d), want (0, 0)", total, failed)
	}
}

func TestNotificationService_Send_Success(t *testing.T) {
	mock := &mockNotifRepo{}
	svc := &NotificationService{repo: mock}

	svc.Send(2, 1, model.NotifTypeLike, 42)

	if len(mock.notifs) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(mock.notifs))
	}
	n := mock.notifs[0]
	if n.UserID != 2 {
		t.Errorf("UserID = %d, want 2", n.UserID)
	}
	if n.ActorID != 1 {
		t.Errorf("ActorID = %d, want 1", n.ActorID)
	}
	if n.Type != model.NotifTypeLike {
		t.Errorf("Type = %s, want %s", n.Type, model.NotifTypeLike)
	}
	if n.TargetID != 42 {
		t.Errorf("TargetID = %d, want 42", n.TargetID)
	}

	total, failed := svc.Stats()
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if failed != 0 {
		t.Errorf("failed = %d, want 0", failed)
	}
}

func TestNotificationService_Send_RepoError(t *testing.T) {
	mock := &mockNotifRepo{err: errors.New("db error")}
	svc := &NotificationService{repo: mock}

	svc.Send(2, 1, model.NotifTypeFollow, 99)

	// Notification should not be recorded
	if len(mock.notifs) != 0 {
		t.Errorf("expected 0 notifications on repo error, got %d", len(mock.notifs))
	}

	// Stats should reflect the failure
	total, failed := svc.Stats()
	if total != 1 {
		t.Errorf("total = %d, want 1 (even failed sends count towards total)", total)
	}
	if failed != 1 {
		t.Errorf("failed = %d, want 1", failed)
	}
}

func TestNotificationService_Stats_MultipleSends(t *testing.T) {
	mock := &mockNotifRepo{}
	svc := &NotificationService{repo: mock}

	// 3 successful sends + 1 self-skip
	svc.Send(2, 1, model.NotifTypeLike, 1)
	svc.Send(3, 1, model.NotifTypeComment, 2)
	svc.Send(1, 1, model.NotifTypeFollow, 3) // self → skipped
	svc.Send(4, 1, model.NotifTypeRepost, 4)

	total, failed := svc.Stats()
	if total != 3 {
		t.Errorf("total = %d, want 3 (self-skip not counted)", total)
	}
	if failed != 0 {
		t.Errorf("failed = %d, want 0", failed)
	}
}

func TestNotificationService_List_Defaults(t *testing.T) {
	mock := &mockNotifRepo{
		notifs: []model.Notification{
			{ID: 1, UserID: 1, Type: model.NotifTypeLike},
		},
	}
	svc := &NotificationService{repo: mock}

	result, total, err := svc.List(1, false, 0, 0)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(result) != 1 {
		t.Errorf("len(result) = %d, want 1", len(result))
	}
	// Verify clamping: page=0→1, pageSize=0→20
	if mock.lastListPage != 1 {
		t.Errorf("page after clamp = %d, want 1", mock.lastListPage)
	}
	if mock.lastListPageSize != 20 {
		t.Errorf("pageSize after clamp = %d, want 20", mock.lastListPageSize)
	}
}

func TestNotificationService_List_PageSizeClamp(t *testing.T) {
	mock := &mockNotifRepo{}
	svc := &NotificationService{repo: mock}

	_, _, err := svc.List(1, true, 1, 100) // pageSize > 50 → clamped
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if mock.lastListPageSize != 20 {
		t.Errorf("pageSize after clamp = %d, want 20", mock.lastListPageSize)
	}
	if !mock.lastListUnreadOnly {
		t.Error("unreadOnly should be true")
	}
}

func TestNotificationService_List_RepoError(t *testing.T) {
	mock := &mockNotifRepo{err: errors.New("db error")}
	svc := &NotificationService{repo: mock}

	_, _, err := svc.List(1, false, 1, 20)
	if err == nil {
		t.Error("expected error from repo, got nil")
	}
}

func TestNotificationService_MarkRead(t *testing.T) {
	mock := &mockNotifRepo{}
	svc := &NotificationService{repo: mock}

	err := svc.MarkRead(42, 7)
	if err != nil {
		t.Fatalf("MarkRead returned error: %v", err)
	}
	if mock.lastMarkReadID != 42 {
		t.Errorf("MarkRead id = %d, want 42", mock.lastMarkReadID)
	}
	if mock.lastMarkReadUserID != 7 {
		t.Errorf("MarkRead userID = %d, want 7", mock.lastMarkReadUserID)
	}
}

func TestNotificationService_MarkRead_Error(t *testing.T) {
	mock := &mockNotifRepo{err: errors.New("db error")}
	svc := &NotificationService{repo: mock}

	err := svc.MarkRead(1, 1)
	if err == nil {
		t.Error("expected error from repo, got nil")
	}
}

func TestNotificationService_MarkAllRead(t *testing.T) {
	mock := &mockNotifRepo{}
	svc := &NotificationService{repo: mock}

	err := svc.MarkAllRead(7)
	if err != nil {
		t.Fatalf("MarkAllRead returned error: %v", err)
	}
	if mock.lastMarkAllReadUserID != 7 {
		t.Errorf("MarkAllRead userID = %d, want 7", mock.lastMarkAllReadUserID)
	}
}

func TestNotificationService_UnreadCount(t *testing.T) {
	mock := &mockNotifRepo{unreadCount: 5}
	svc := &NotificationService{repo: mock}

	count := svc.UnreadCount(7)
	if count != 5 {
		t.Errorf("UnreadCount = %d, want 5", count)
	}
	if mock.lastUnreadCountUserID != 7 {
		t.Errorf("UnreadCount userID = %d, want 7", mock.lastUnreadCountUserID)
	}
}
