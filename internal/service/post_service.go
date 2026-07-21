package service

import (
	"errors"
	"log/slog"

	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/repository"
	"gorm.io/gorm"
)

type PostService struct {
	postRepo     *repository.PostRepo
	topicRepo    *repository.TopicRepo
	likeRepo     *repository.LikeRepo
	favoriteRepo *repository.FavoriteRepo
	feedSvc      *FeedService
	notifSvc     *NotificationService
}

func NewPostService() *PostService {
	return &PostService{
		postRepo:     repository.NewPostRepo(),
		topicRepo:    repository.NewTopicRepo(),
		likeRepo:     repository.NewLikeRepo(),
		favoriteRepo: repository.NewFavoriteRepo(),
		feedSvc:      NewFeedService(),
		notifSvc:     NewNotificationService(),
	}
}

type CreatePostReq struct {
	Content  string   `json:"content"`
	Images   []string `json:"images"` // image URLs from upload
	TopicIDs []int64  `json:"topic_ids"`
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

	// Create post and associate topics in a single transaction to avoid orphan posts
	err := repository.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(post).Error; err != nil {
			return err
		}
		topicIDs := s.resolveTopicIDsInTx(tx, req.Content, req.TopicIDs)
		return s.topicRepo.ReplacePostTopics(tx, post.ID, topicIDs)
	})
	if err != nil {
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

	// Re-associate hashtag topics in a transaction (resolve + clear old + re-add)
	if err := repository.DB.Transaction(func(tx *gorm.DB) error {
		// Resolve topic IDs within transaction to avoid orphan topics
		topicIDs := s.resolveTopicIDsInTx(tx, content, nil)
		return s.topicRepo.ReplacePostTopics(tx, postID, topicIDs)
	}); err != nil {
		slog.Warn("failed to re-associate post topics", "post_id", postID, "err", err)
	}

	// Re-fetch to get fresh data (updated_at, etc.)
	return s.postRepo.FindByID(postID)
}

// resolveTopicIDsInTx resolves topic IDs within a transaction to avoid orphan topics.
func (s *PostService) resolveTopicIDsInTx(tx *gorm.DB, content string, explicit []int64) []int64 {
	seen := make(map[int64]bool)
	var ids []int64

	for _, tag := range ParseHashtags(content) {
		topic, _, err := s.topicRepo.FindOrCreateWithTx(tx, tag)
		if err != nil {
			slog.Warn("failed to resolve post topic", "topic", tag, "err", err)
			continue
		}
		if topic != nil && !seen[topic.ID] {
			seen[topic.ID] = true
			ids = append(ids, topic.ID)
		}
	}

	for _, tid := range explicit {
		if !seen[tid] {
			seen[tid] = true
			ids = append(ids, tid)
		}
	}

	return ids
}

type RepostReq struct {
	Text   string   `json:"text"`
	Images []string `json:"images"`
}

func (s *PostService) Repost(userID, originalPostID int64, req RepostReq) (*model.Post, error) {
	original, err := s.postRepo.FindByID(originalPostID)
	if err != nil || original == nil || original.IsDeleted == 1 || original.Status != model.StatusApproved {
		return nil, errors.New("原帖不存在")
	}

	coverImage := original.CoverImage
	if len(req.Images) > 0 {
		coverImage = req.Images[0]
	}

	post := &model.Post{
		UserID:     userID,
		Content:    req.Text,
		CoverImage: coverImage,
		IsRepost:   1,
		RepostOfID: &originalPostID,
		RepostText: req.Text,
		Status:     model.StatusPending,
	}

	for i, url := range req.Images {
		post.Images = append(post.Images, model.PostImage{ImageURL: url, SortOrder: i})
	}

	if err := s.postRepo.Create(post); err != nil {
		return nil, err
	}

	s.postRepo.IncrementShare(originalPostID)
	s.feedSvc.InvalidateCache()
	s.notifSvc.Send(original.UserID, userID, model.NotifTypeRepost, originalPostID)
	return s.postRepo.FindByID(post.ID)
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
		post.IsFavorited = s.favoriteRepo.IsFavorited(currentUserID, postID)
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
