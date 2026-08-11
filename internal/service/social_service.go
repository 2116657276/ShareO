package service

import (
	"context"
	"errors"
	"strings"

	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/repository"
	"gorm.io/gorm"
)

type SocialService struct {
	likeRepo        *repository.LikeRepo
	commentLikeRepo *repository.CommentLikeRepo
	followRepo      *repository.FollowRepo
	commentRepo     *repository.CommentRepo
	userRepo        *repository.UserRepo
	postRepo        *repository.PostRepo
	notifSvc        *NotificationService
}

// Sentinel errors for handler-layer HTTP status decisions
var (
	ErrPostNotFound         = errors.New("帖子不存在")
	ErrPermissionDenied     = errors.New("无权操作")
	ErrFollowSelf           = errors.New("cannot follow yourself")
	ErrCommentInvalid       = errors.New("评论内容无效")
	ErrCommentParentInvalid = errors.New("评论父级无效")
	ErrCommentReplyInvalid  = errors.New("评论回复对象无效")
)

func NewSocialService() *SocialService {
	return &SocialService{
		likeRepo:        repository.NewLikeRepo(),
		commentLikeRepo: repository.NewCommentLikeRepo(),
		followRepo:      repository.NewFollowRepo(),
		commentRepo:     repository.NewCommentRepo(),
		userRepo:        repository.NewUserRepo(),
		postRepo:        repository.NewPostRepo(),
		notifSvc:        NewNotificationService(),
	}
}

// --- Like ---

func (s *SocialService) ToggleLike(userID, postID int64) (bool, error) {
	post, err := s.postRepo.FindByIDLight(postID)
	if err != nil || post == nil || post.IsDeleted == 1 || post.Status != model.StatusApproved {
		return false, ErrPostNotFound
	}
	liked, err := s.likeRepo.Toggle(userID, postID)
	if err == nil && liked {
		s.notifSvc.Send(post.UserID, userID, model.NotifTypeLike, postID)
	}
	return liked, err
}

func (s *SocialService) GetLikedPosts(userID int64, page, pageSize int) ([]model.Post, int64, error) {
	page, pageSize = clampPage(page, pageSize)
	return s.likeRepo.GetUserLikedPosts(userID, page, pageSize)
}

// --- Follow ---

func (s *SocialService) ToggleFollow(followerID, followeeID int64) (bool, error) {
	if followerID == followeeID {
		return false, ErrFollowSelf
	}
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
	page, pageSize = clampPage(page, pageSize)
	return s.followRepo.GetFollowing(userID, page, pageSize)
}

func (s *SocialService) GetFollowers(userID int64, page, pageSize int) ([]model.User, int64, error) {
	page, pageSize = clampPage(page, pageSize)
	return s.followRepo.GetFollowers(userID, page, pageSize)
}

// clampPage returns sanitized page and pageSize values.
func clampPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	return page, pageSize
}

// --- Comment ---

type CreateCommentReq struct {
	PostID     int64  `json:"post_id"`
	Content    string `json:"content" binding:"required,min=1,max=500"`
	ParentID   *int64 `json:"parent_id"`
	ReplyToUID *int64 `json:"reply_to_uid"`
}

func (s *SocialService) CreateComment(ctx context.Context, userID int64, req CreateCommentReq) (*model.Comment, error) {
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" || len([]rune(req.Content)) > 500 {
		return nil, ErrCommentInvalid
	}

	post, err := s.postRepo.FindByIDLight(req.PostID)
	if err != nil || post == nil || post.IsDeleted == 1 || post.Status != model.StatusApproved {
		return nil, ErrPostNotFound
	}
	if req.ParentID != nil {
		if *req.ParentID <= 0 {
			return nil, ErrCommentParentInvalid
		}
		parent, err := s.commentRepo.FindVisibleTopLevelForPost(ctx, post.ID, *req.ParentID)
		if err != nil {
			return nil, err
		}
		if parent == nil {
			return nil, ErrCommentParentInvalid
		}
	}

	// Validate reply_to_uid: target user must exist and be involved in the discussion
	if req.ReplyToUID != nil && *req.ReplyToUID > 0 {
		target, err := s.userRepo.FindByID(*req.ReplyToUID)
		if err != nil {
			return nil, ErrCommentReplyInvalid
		}
		if target == nil {
			return nil, ErrCommentReplyInvalid
		}
		// Only allow replying to post author or existing commenters
		if *req.ReplyToUID != post.UserID {
			hasComment, err := s.commentRepo.HasUserCommented(post.ID, *req.ReplyToUID)
			if err != nil {
				return nil, err
			}
			if !hasComment {
				return nil, ErrCommentReplyInvalid
			}
		}
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

func (s *SocialService) GetCommentsForUser(postID int64, page, pageSize int, currentUserID int64) ([]model.Comment, int64, error) {
	return s.commentRepo.FindByPostIDForUser(postID, page, pageSize, currentUserID)
}

func (s *SocialService) ToggleCommentLike(userID, commentID int64) (bool, int, error) {
	comment, err := s.commentRepo.FindVisibleByID(commentID)
	if err != nil {
		return false, 0, err
	}
	if comment == nil {
		return false, 0, ErrPostNotFound
	}
	return s.commentLikeRepo.Toggle(userID, commentID)
}

func (s *SocialService) DeleteComment(userID, commentID int64) error {
	err := s.commentRepo.SoftDelete(commentID, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("评论不存在或无权删除")
	}
	return err
}
