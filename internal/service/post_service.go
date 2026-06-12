package service

import (
	"errors"

	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

type PostService struct {
	postRepo     *repository.PostRepo
	topicRepo    *repository.TopicRepo
	likeRepo     *repository.LikeRepo
	favoriteRepo *repository.FavoriteRepo
	feedSvc      *FeedService
}

func NewPostService() *PostService {
	return &PostService{
		postRepo:     repository.NewPostRepo(),
		topicRepo:    repository.NewTopicRepo(),
		likeRepo:     repository.NewLikeRepo(),
		favoriteRepo: repository.NewFavoriteRepo(),
		feedSvc:      NewFeedService(),
	}
}

type CreatePostReq struct {
	Content string   `json:"content"`
	Images  []string `json:"images"`  // image URLs from upload
	TopicIDs []int64 `json:"topic_ids"`
}

func (s *PostService) Create(userID int64, req CreatePostReq) (*model.Post, error) {
	if len(req.Images) == 0 {
		return nil, errors.New("至少需要上传一张图片")
	}

	post := &model.Post{
		UserID:     userID,
		Content:    req.Content,
		CoverImage: req.Images[0],
		Status:     "pending", // 需要管理员审核
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

	// 关联话题
	for _, tid := range req.TopicIDs {
		_ = s.topicRepo.AddPostToTopic(tid, post.ID)
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
	// re-submit for review after edit
	post.Status = "pending"
	if err := s.postRepo.Update(post); err != nil {
		return nil, err
	}
	return post, nil
}

type RepostReq struct {
	Text   string   `json:"text"`
	Images []string `json:"images"`
}

func (s *PostService) Repost(userID, originalPostID int64, req RepostReq) (*model.Post, error) {
	original, err := s.postRepo.FindByID(originalPostID)
	if err != nil || original == nil || original.IsDeleted == 1 {
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
		Status:     "pending",
	}

	for i, url := range req.Images {
		post.Images = append(post.Images, model.PostImage{ImageURL: url, SortOrder: i})
	}

	if err := s.postRepo.Create(post); err != nil {
		return nil, err
	}

	s.postRepo.IncrementShare(originalPostID)
	s.feedSvc.InvalidateCache()
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

	s.postRepo.IncrementView(postID)

	if currentUserID > 0 {
		post.IsLiked = s.likeRepo.IsLiked(currentUserID, postID)
		post.IsFavorited = s.favoriteRepo.IsFavorited(currentUserID, postID)
	}
	return post, nil
}
