package service

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

type PostService struct {
	postRepo     *repository.PostRepo
	likeRepo     *repository.LikeRepo
	favoriteRepo *repository.FavoriteRepo
	feedSvc      *FeedService
}

const (
	MaxPostContentLength = 500
	MaxPostImages        = 9
)

var ErrPostInvalid = errors.New("帖子参数无效")

func NewPostService() *PostService {
	return &PostService{
		postRepo:     repository.NewPostRepo(),
		likeRepo:     repository.NewLikeRepo(),
		favoriteRepo: repository.NewFavoriteRepo(),
		feedSvc:      NewFeedService(),
	}
}

type CreatePostReq struct {
	Content string   `json:"content"`
	Images  []string `json:"images"` // image URLs from upload
}

func (s *PostService) Create(userID int64, req CreatePostReq) (*model.Post, error) {
	content, images, err := validatePostInput(req.Content, req.Images)
	if err != nil {
		return nil, err
	}

	post := &model.Post{
		UserID:     userID,
		Content:    content,
		CoverImage: images[0],
		Status:     model.StatusPending, // 需要管理员审核
	}

	// 构建图片关联
	for i, imageURL := range images {
		post.Images = append(post.Images, model.PostImage{
			ImageURL:  imageURL,
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
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, ErrPostNotFound
	}
	if post.UserID != userID {
		return nil, ErrPermissionDenied
	}
	content, err = validatePostContent(content)
	if err != nil {
		return nil, err
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
	if err != nil {
		return err
	}
	if post == nil {
		return ErrPostNotFound
	}
	if post.UserID != userID {
		return ErrPermissionDenied
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
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, ErrPostNotFound
	}
	if post.IsDeleted == 1 {
		return nil, ErrPostNotFound
	}

	// Hide pending/rejected posts from non-owner non-admin users
	if post.Status != model.StatusApproved && post.UserID != currentUserID {
		return nil, ErrPostNotFound
	}

	if currentUserID > 0 {
		post.IsLiked = s.likeRepo.IsLiked(currentUserID, postID)
		post.IsFavorited = s.favoriteRepo.IsFavorited(context.Background(), currentUserID, postID)
	}
	return post, nil
}

func validatePostInput(content string, images []string) (string, []string, error) {
	content, err := validatePostContent(content)
	if err != nil {
		return "", nil, err
	}
	if len(images) == 0 || len(images) > MaxPostImages {
		return "", nil, ErrPostInvalid
	}

	normalized := make([]string, len(images))
	seen := make(map[string]struct{}, len(images))
	for i, raw := range images {
		imageURL := strings.TrimSpace(raw)
		if !validUploadedImageURL(imageURL) {
			return "", nil, ErrPostInvalid
		}
		if _, exists := seen[imageURL]; exists {
			return "", nil, ErrPostInvalid
		}
		seen[imageURL] = struct{}{}
		normalized[i] = imageURL
	}
	return content, normalized, nil
}

func validatePostContent(content string) (string, error) {
	content = strings.TrimSpace(content)
	if len([]rune(content)) > MaxPostContentLength {
		return "", ErrPostInvalid
	}
	return content, nil
}

func validUploadedImageURL(raw string) bool {
	if raw == "" || strings.ContainsAny(raw, "\\\x00") {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	path := parsed.EscapedPath()
	return strings.HasPrefix(path, "/api/v1/images/posts/") &&
		!strings.Contains(path, "..") && len(path) > len("/api/v1/images/posts/")
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
