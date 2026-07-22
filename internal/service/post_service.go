package service

import (
	"errors"

	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

type PostService struct {
	postRepo *repository.PostRepo
	likeRepo *repository.LikeRepo
	feedSvc  *FeedService
}

func NewPostService() *PostService {
	return &PostService{
		postRepo: repository.NewPostRepo(),
		likeRepo: repository.NewLikeRepo(),
		feedSvc:  NewFeedService(),
	}
}

type CreatePostReq struct {
	Content string   `json:"content"`
	Images  []string `json:"images"` // image URLs from upload
}

func (s *PostService) Create(userID int64, req CreatePostReq) (*model.Post, error) {
	if len(req.Images) == 0 {
		return nil, errors.New("至少需要上传一张图片")
	}

	post := &model.Post{
		UserID:     userID,
		Content:    req.Content,
		CoverImage: req.Images[0],
		Status:     model.StatusPending, // 需要管理员审核
	}

	// 构建图片关联
	for i, url := range req.Images {
		post.Images = append(post.Images, model.PostImage{
			ImageURL:  url,
			SortOrder: i,
		})
	}

	if err := s.postRepo.Create(post); err != nil {
		return nil, err
	}

	s.feedSvc.InvalidateCache()
	return s.postRepo.FindByID(post.ID)
}

func (s *PostService) Update(userID, postID int64, content string) (*model.Post, error) {
	post, err := s.postRepo.FindByID(postID)
	if err != nil || post == nil {
		return nil, errors.New("帖子不存在")
	}
	if post.UserID != userID {
		return nil, errors.New("无权编辑此帖子")
	}

	post.Content = content
	post.Status = model.StatusPending
	if err := s.postRepo.Update(post); err != nil {
		return nil, err
	}
	publishIndexAction("delete", postID)

	// Re-fetch to get fresh data (updated_at, etc.)
	return s.postRepo.FindByID(postID)
}

func (s *PostService) Delete(userID, postID int64) error {
	post, err := s.postRepo.FindByID(postID)
	if err != nil || post == nil {
		return errors.New("帖子不存在")
	}
	if post.UserID != userID {
		return errors.New("无权删除此帖子")
	}
	err = s.postRepo.SoftDelete(postID, userID)
	if err == nil {
		publishIndexAction("delete", postID)
		s.feedSvc.InvalidateCache()
	}
	return err
}

func (s *PostService) GetByID(postID int64, currentUserID int64) (*model.Post, error) {
	post, err := s.postRepo.FindByID(postID)
	if err != nil || post == nil {
		return nil, errors.New("帖子不存在")
	}
	if post.IsDeleted == 1 {
		return nil, errors.New("帖子已被删除")
	}

	// Hide pending/rejected posts from non-owner non-admin users
	if post.Status != model.StatusApproved && post.UserID != currentUserID {
		return nil, errors.New("帖子不存在")
	}

	if currentUserID > 0 {
		post.IsLiked = s.likeRepo.IsLiked(currentUserID, postID)
	}
	return post, nil
}

// RecordView is deliberately separate from GET so that rendering and API reads
// remain side-effect free. Repeated views are still counted by the existing
// product policy, except for an author's own post.
func (s *PostService) RecordView(postID, currentUserID int64) error {
	post, err := s.GetByID(postID, currentUserID)
	if err != nil {
		return err
	}
	if post.UserID != currentUserID {
		s.postRepo.IncrementView(postID)
	}
	return nil
}
