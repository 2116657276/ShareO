package service

import (
	"errors"

	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/repository"
	"gorm.io/gorm"
)

type SocialService struct {
	likeRepo     *repository.LikeRepo
	favoriteRepo *repository.FavoriteRepo
	followRepo   *repository.FollowRepo
	commentRepo  *repository.CommentRepo
	userRepo     *repository.UserRepo
	postRepo     *repository.PostRepo
	notifSvc     *NotificationService
}

// Sentinel errors for handler-layer HTTP status decisions
var (
	ErrPostNotFound     = errors.New("帖子不存在")
	ErrPermissionDenied = errors.New("无权操作")
)

func NewSocialService() *SocialService {
	return &SocialService{
		likeRepo:     repository.NewLikeRepo(),
		favoriteRepo: repository.NewFavoriteRepo(),
		followRepo:   repository.NewFollowRepo(),
		commentRepo:  repository.NewCommentRepo(),
		userRepo:     repository.NewUserRepo(),
		postRepo:     repository.NewPostRepo(),
		notifSvc:     NewNotificationService(),
	}
}

// --- Like ---

func (s *SocialService) ToggleLike(userID, postID int64) (bool, error) {
	post, err := s.postRepo.FindByIDLight(postID)
	if err != nil || post == nil || post.IsDeleted == 1 {
		return false, ErrPostNotFound
	}
	liked, err := s.likeRepo.Toggle(userID, postID)
	if err == nil && liked {
		s.notifSvc.Send(post.UserID, userID, model.NotifTypeLike, postID)
	}
	return liked, err
}

func (s *SocialService) GetLikedPosts(userID int64, page, pageSize int) ([]model.Post, int64, error) {
	return s.likeRepo.GetUserLikedPosts(userID, page, pageSize)
}

// --- Favorite ---

func (s *SocialService) ToggleFavorite(userID, postID int64) (bool, error) {
	post, err := s.postRepo.FindByIDLight(postID)
	if err != nil || post == nil || post.IsDeleted == 1 {
		return false, ErrPostNotFound
	}
	return s.favoriteRepo.Toggle(userID, postID)
}

func (s *SocialService) GetFavorites(userID int64, page, pageSize int) ([]model.Post, int64, error) {
	return s.favoriteRepo.GetUserFavorites(userID, page, pageSize)
}

// --- Follow ---

func (s *SocialService) ToggleFollow(followerID, followeeID int64) (bool, error) {
	following, err := s.followRepo.Toggle(followerID, followeeID)
	if err == nil && following {
		s.notifSvc.Send(followeeID, followerID, model.NotifTypeFollow, 0)
	}
	return following, err
}

func (s *SocialService) IsFollowing(followerID, followeeID int64) bool {
	return s.followRepo.IsFollowing(followerID, followeeID)
}

func (s *SocialService) GetFollowing(userID int64, page, pageSize int) ([]model.User, int64, error) {
	return s.followRepo.GetFollowing(userID, page, pageSize)
}

func (s *SocialService) GetFollowers(userID int64, page, pageSize int) ([]model.User, int64, error) {
	return s.followRepo.GetFollowers(userID, page, pageSize)
}

// --- Comment ---

type CreateCommentReq struct {
	PostID     int64  `json:"post_id"`
	Content    string `json:"content" binding:"required,min=1,max=500"`
	ParentID   *int64 `json:"parent_id"`
	ReplyToUID *int64 `json:"reply_to_uid"`
}

func (s *SocialService) CreateComment(userID int64, req CreateCommentReq) (*model.Comment, error) {
	if req.Content == "" {
		return nil, errors.New("评论内容不能为空")
	}

	post, err := s.postRepo.FindByIDLight(req.PostID)
	if err != nil || post == nil || post.IsDeleted == 1 {
		return nil, ErrPostNotFound
	}

	comment := &model.Comment{
		PostID:     req.PostID,
		UserID:     userID,
		ParentID:   req.ParentID,
		ReplyToUID: req.ReplyToUID,
		Content:    req.Content,
	}
	if err := s.commentRepo.Create(comment); err != nil {
		return nil, err
	}
	// Notify post owner (only if commenter is not the post owner)
	s.notifSvc.Send(post.UserID, userID, model.NotifTypeComment, req.PostID)
	// Notify the replied-to user (for nested replies)
	if req.ReplyToUID != nil && *req.ReplyToUID != userID && *req.ReplyToUID != post.UserID {
		s.notifSvc.Send(*req.ReplyToUID, userID, model.NotifTypeComment, req.PostID)
	}
	return comment, nil
}

func (s *SocialService) GetComments(postID int64, page, pageSize int) ([]model.Comment, int64, error) {
	return s.commentRepo.FindByPostID(postID, page, pageSize)
}

func (s *SocialService) DeleteComment(userID, commentID int64) error {
	err := s.commentRepo.SoftDelete(commentID, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("评论不存在或无权删除")
	}
	return err
}
