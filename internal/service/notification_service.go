package service

import (
	"log/slog"
	"sync/atomic"

	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

// notifRepo defines the methods NotificationService needs from the repository layer.
// *repository.NotificationRepo satisfies this interface automatically.
type notifRepo interface {
	Create(notif *model.Notification) error
	List(userID int64, unreadOnly bool, page, pageSize int) ([]model.Notification, int64, error)
	MarkRead(id, userID int64) error
	MarkAllRead(userID int64) error
	UnreadCount(userID int64) int64
}

type NotificationService struct {
	repo       notifRepo
	failCount  int64 // atomic counter for failed sends
	totalCount int64 // atomic counter for total send attempts
}

func NewNotificationService() *NotificationService {
	return &NotificationService{repo: repository.NewNotificationRepo()}
}

// Send creates a notification for the target user.
// Skips self-notifications (actorID == userID).
func (s *NotificationService) Send(userID, actorID int64, notifType string, targetID int64) {
	if userID == actorID {
		return // don't notify self
	}
	notif := &model.Notification{
		UserID:   userID,
		Type:     notifType,
		ActorID:  actorID,
		TargetID: targetID,
	}
	if err := s.repo.Create(notif); err != nil {
		slog.Warn("failed to create notification", "user_id", userID, "type", notifType, "err", err)
		atomic.AddInt64(&s.failCount, 1)
	}
	atomic.AddInt64(&s.totalCount, 1)
}

func (s *NotificationService) List(userID int64, unreadOnly bool, page, pageSize int) ([]model.Notification, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	return s.repo.List(userID, unreadOnly, page, pageSize)
}

func (s *NotificationService) MarkRead(id, userID int64) error {
	return s.repo.MarkRead(id, userID)
}

func (s *NotificationService) MarkAllRead(userID int64) error {
	return s.repo.MarkAllRead(userID)
}

func (s *NotificationService) UnreadCount(userID int64) int64 {
	return s.repo.UnreadCount(userID)
}
