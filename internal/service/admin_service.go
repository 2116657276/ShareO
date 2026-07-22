package service

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

type AdminService struct {
	postRepo    *repository.PostRepo
	userRepo    *repository.UserRepo
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
		logRepo:     repository.NewLogRepo(),
		likeRepo:    repository.NewLikeRepo(),
		commentRepo: repository.NewCommentRepo(),
		feedSvc:     NewFeedService(),
		notifSvc:    NewNotificationService(),
	}
}

// --- Post Review ---

func (s *AdminService) DeletePost(postID int64, adminID int64) error {
	// Fetch post info before deletion to get author ID for notification
	post, err := s.postRepo.FindByIDLight(postID)
	if err != nil {
		return err
	}
	if post == nil {
		return errors.New("帖子不存在")
	}

	if err := s.postRepo.AdminSoftDelete(postID); err != nil {
		return err
	}
	publishIndexAction("delete", postID)
	s.feedSvc.InvalidateCache()
	// Notify the author about forced deletion
	s.notifSvc.Send(post.UserID, adminID, model.NotifTypeReview, postID)
	s.writeAdminLog(adminID, "delete_post", fmt.Sprintf("post_id=%d", postID))
	return nil
}

func (s *AdminService) ReviewPost(postID int64, status, comment string, reviewerID int64) error {
	if status != model.StatusApproved && status != model.StatusRejected {
		return errors.New("审核状态无效")
	}
	err := s.postRepo.UpdateStatus(postID, status, comment, reviewerID)
	if err == nil {
		if status == model.StatusApproved {
			publishIndexAction("upsert", postID)
		} else {
			publishIndexAction("delete", postID)
		}
		s.feedSvc.InvalidateCache()
		// Notify the post author about review result
		if post, findErr := s.postRepo.FindByID(postID); findErr == nil && post != nil {
			s.notifSvc.Send(post.UserID, reviewerID, model.NotifTypeReview, postID)
		}
		s.writeAdminLog(reviewerID, "review_post", fmt.Sprintf("post_id=%d status=%s", postID, status))
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

func (s *AdminService) UpdateUserStatus(adminID, targetUserID int64, status int8) error {
	if adminID == targetUserID {
		return errors.New("不能封禁自己")
	}
	targetUser, err := s.userRepo.FindByID(targetUserID)
	if err != nil {
		return err
	}
	if targetUser == nil {
		return errors.New("用户不存在")
	}
	if targetUser.Role == "admin" {
		return errors.New("不能封禁其他管理员")
	}
	if status != model.UserStatusBanned && status != model.UserStatusActive {
		return errors.New("状态值无效")
	}
	if err := s.userRepo.UpdateStatus(targetUserID, status); err != nil {
		return err
	}
	s.writeAdminLog(adminID, "update_user_status", fmt.Sprintf("user_id=%d status=%d", targetUserID, status))
	return nil
}

func (s *AdminService) writeAdminLog(adminID int64, action, detail string) {
	if err := s.logRepo.Create(&model.SystemLog{UserID: &adminID, Action: action, Detail: detail}); err != nil {
		slog.Warn("failed to write admin audit log", "admin_id", adminID, "action", action, "err", err)
	}
}

// --- Logs ---

func (s *AdminService) GetLogs(page, pageSize int, userID *int64, action string) ([]model.SystemLog, int64, error) {
	return s.logRepo.List(page, pageSize, userID, action)
}

// --- Stats ---

type DashboardStats struct {
	TotalUsers    int64 `json:"total_users"`
	TotalPosts    int64 `json:"total_posts"`
	PendingPosts  int64 `json:"pending_posts"`
	TotalLikes    int64 `json:"total_likes"`
	TotalComments int64 `json:"total_comments"`
	BannedUsers   int64 `json:"banned_users"`
}

func (s *AdminService) GetDashboardStats() (*DashboardStats, error) {
	stats := &DashboardStats{}

	stats.TotalUsers = s.userRepo.CountByRole(model.RoleUser)
	stats.BannedUsers = s.userRepo.CountByStatus(model.UserStatusBanned)
	stats.TotalPosts = s.postRepo.CountTotal()
	stats.PendingPosts = s.postRepo.CountByStatus(model.StatusPending)
	stats.TotalLikes = s.likeRepo.CountTotal()
	stats.TotalComments = s.commentRepo.CountNonDeleted()
	return stats, nil
}
