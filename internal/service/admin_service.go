package service

import (
	"errors"

	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

type AdminService struct {
	postRepo    *repository.PostRepo
	userRepo    *repository.UserRepo
	topicRepo   *repository.TopicRepo
	logRepo     *repository.LogRepo
	likeRepo    *repository.LikeRepo
	commentRepo *repository.CommentRepo
	feedSvc     *FeedService
	notifSvc    *NotificationService
}

func NewAdminService() *AdminService {
	return &AdminService{
		postRepo:    repository.NewPostRepo(),
		userRepo:    repository.NewUserRepo(),
		topicRepo:   repository.NewTopicRepo(),
		logRepo:     repository.NewLogRepo(),
		likeRepo:    repository.NewLikeRepo(),
		commentRepo: repository.NewCommentRepo(),
		feedSvc:     NewFeedService(),
		notifSvc:    NewNotificationService(),
	}
}

// --- Post Review ---

func (s *AdminService) DeletePost(postID int64) error {
	if err := s.postRepo.AdminSoftDelete(postID); err != nil {
		return err
	}
	s.feedSvc.InvalidateCache()
	return nil
}

func (s *AdminService) ReviewPost(postID int64, status, comment string, reviewerID int64) error {
	if status != model.StatusApproved && status != model.StatusRejected {
		return errors.New("审核状态无效")
	}
	err := s.postRepo.UpdateStatus(postID, status, comment, reviewerID)
	if err == nil {
		s.feedSvc.InvalidateCache()
		// Notify the post author about review result
		if post, findErr := s.postRepo.FindByID(postID); findErr == nil && post != nil {
			s.notifSvc.Send(post.UserID, reviewerID, model.NotifTypeReview, postID)
		}
	}
	return err
}

func (s *AdminService) GetPendingPosts(page, pageSize int) ([]model.Post, int64, error) {
	q := repository.FeedQuery{
		Status:   model.StatusPending,
		Page:     page,
		PageSize: pageSize,
		Sort:     model.SortLatest,
	}
	return s.postRepo.Feed(q)
}

// --- User Management ---

func (s *AdminService) ListUsers(page, pageSize int, role string, status *int) ([]model.User, int64, error) {
	return s.userRepo.List(page, pageSize, role, status)
}

func (s *AdminService) UpdateUserStatus(userID int64, status int8) error {
	if status != model.UserStatusBanned && status != model.UserStatusActive {
		return errors.New("状态值无效")
	}
	return s.userRepo.UpdateStatus(userID, status)
}

// --- Topic Management ---

func (s *AdminService) CreateTopic(name, description string) (*model.Topic, error) {
	topic := &model.Topic{
		Name:        name,
		Description: description,
		Status:      1,
	}
	if err := s.topicRepo.Create(topic); err != nil {
		return nil, err
	}
	return topic, nil
}

func (s *AdminService) UpdateTopic(id int64, name, description string, status int8) error {
	topic, err := s.topicRepo.FindByID(id)
	if err != nil {
		return err
	}
	if name != "" {
		topic.Name = name
	}
	if description != "" {
		topic.Description = description
	}
	topic.Status = status
	return s.topicRepo.Update(topic)
}

func (s *AdminService) DeleteTopic(id int64) error {
	return s.topicRepo.Delete(id)
}

// --- Logs ---

func (s *AdminService) GetLogs(page, pageSize int, userID *int64, action string) ([]model.SystemLog, int64, error) {
	return s.logRepo.List(page, pageSize, userID, action)
}

// --- Stats ---

type DashboardStats struct {
	TotalUsers       int64 `json:"total_users"`
	TotalPosts       int64 `json:"total_posts"`
	PendingPosts     int64 `json:"pending_posts"`
	TotalLikes       int64 `json:"total_likes"`
	TotalComments    int64 `json:"total_comments"`
	ActiveTopics     int64 `json:"active_topics"`
	BannedUsers      int64 `json:"banned_users"`
}

func (s *AdminService) GetDashboardStats() (*DashboardStats, error) {
	stats := &DashboardStats{}

	stats.TotalUsers = s.userRepo.CountByRole(model.RoleUser)
	stats.BannedUsers = s.userRepo.CountByStatus(model.UserStatusBanned)
	stats.TotalPosts = s.postRepo.CountTotal()
	stats.PendingPosts = s.postRepo.CountByStatus(model.StatusPending)
	stats.TotalLikes = s.likeRepo.CountTotal()
	stats.TotalComments = s.commentRepo.CountNonDeleted()
	stats.ActiveTopics = s.topicRepo.CountByStatus(model.TopicStatusActive)

	return stats, nil
}
