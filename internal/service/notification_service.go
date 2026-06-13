package service

import (
	"log"
	"sync/atomic"

	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

type NotificationService struct {
	repo       *repository.NotificationRepo
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
		log.Printf("NotificationService.Send: failed to create notification (user=%d, type=%s): %v", userID, notifType, err)
		atomic.AddInt64(&s.failCount, 1)
	}
	atomic.AddInt64(&s.totalCount, 1)
}

// Stats returns (total send attempts, failed sends) for observability.
func (s *NotificationService) Stats() (total, failed int64) {
	return atomic.LoadInt64(&s.totalCount), atomic.LoadInt64(&s.failCount)
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
