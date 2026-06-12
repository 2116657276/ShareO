package service

import (
	"errors"

	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

type AdminService struct {
	postRepo  *repository.PostRepo
	userRepo  *repository.UserRepo
	topicRepo *repository.TopicRepo
	logRepo   *repository.LogRepo
	feedSvc   *FeedService
}

func NewAdminService() *AdminService {
	return &AdminService{
		postRepo:  repository.NewPostRepo(),
		userRepo:  repository.NewUserRepo(),
		topicRepo: repository.NewTopicRepo(),
		logRepo:   repository.NewLogRepo(),
		feedSvc:   NewFeedService(),
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
	if status != "approved" && status != "rejected" {
		return errors.New("审核状态无效")
	}
	err := s.postRepo.UpdateStatus(postID, status, comment, reviewerID)
	if err == nil {
		s.feedSvc.InvalidateCache()
	}
	return err
}

func (s *AdminService) GetPendingPosts(page, pageSize int) ([]model.Post, int64, error) {
	q := repository.FeedQuery{
		Status:   "pending",
		Page:     page,
		PageSize: pageSize,
		Sort:     "latest",
	}
	return s.postRepo.Feed(q)
}

// --- User Management ---

func (s *AdminService) ListUsers(page, pageSize int, role string, status *int) ([]model.User, int64, error) {
	return s.userRepo.List(page, pageSize, role, status)
}

func (s *AdminService) UpdateUserStatus(userID int64, status int8) error {
	if status != 0 && status != 1 {
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

	repository.DB.Model(&model.User{}).Where("role = 'user'").Count(&stats.TotalUsers)
	repository.DB.Model(&model.User{}).Where("status = 0").Count(&stats.BannedUsers)
	repository.DB.Model(&model.Post{}).Where("is_deleted = 0").Count(&stats.TotalPosts)
	repository.DB.Model(&model.Post{}).Where("status = 'pending' AND is_deleted = 0").Count(&stats.PendingPosts)
	repository.DB.Model(&model.Like{}).Count(&stats.TotalLikes)
	repository.DB.Model(&model.Comment{}).Where("is_deleted = 0").Count(&stats.TotalComments)
	repository.DB.Model(&model.Topic{}).Where("status = 1").Count(&stats.ActiveTopics)

	return stats, nil
}
