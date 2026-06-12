package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

type FeedService struct {
	postRepo  *repository.PostRepo
	likeRepo  *repository.LikeRepo
	favRepo   *repository.FavoriteRepo
}

func NewFeedService() *FeedService {
	return &FeedService{
		postRepo: repository.NewPostRepo(),
		likeRepo: repository.NewLikeRepo(),
		favRepo:  repository.NewFavoriteRepo(),
	}
}

type FeedReq struct {
	Sort     string `form:"sort"`     // "latest" (default) or "hot"
	TopicID  *int64 `form:"topic_id"`
	UserID   *int64 `form:"user_id"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

func (s *FeedService) GetFeed(req FeedReq, currentUserID int64) ([]model.Post, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 50 {
		req.PageSize = 20
	}
	if req.Sort == "" {
		req.Sort = "latest"
	}

	// Try Redis cache for first page of latest feed (hot feed always fresh)
	if req.Page == 1 && req.Sort == "latest" && req.TopicID == nil && req.UserID == nil {
		cached, cachedTotal, ok := s.getCachedFeed()
		if ok && len(cached) >= req.PageSize {
			// Slice to requested page_size
			result := cached
			if len(result) > req.PageSize {
				result = result[:req.PageSize]
			}
			s.fillUserInteraction(result, currentUserID)
			return result, cachedTotal, nil
		}
	}

	q := repository.FeedQuery{
		UserID:   req.UserID,
		TopicID:  req.TopicID,
		Sort:     req.Sort,
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	posts, total, err := s.postRepo.Feed(q)
	if err != nil {
		return nil, 0, err
	}

	s.fillUserInteraction(posts, currentUserID)

	// Cache first page of latest feed (store page_size=20 worth + total)
	if req.Page == 1 && req.Sort == "latest" && req.TopicID == nil && req.UserID == nil {
		s.cacheFeed(posts, total)
	}

	return posts, total, nil
}

func (s *FeedService) Search(q string, page, pageSize int, currentUserID int64) ([]model.Post, int64, error) {
	posts, total, err := s.postRepo.Search(q, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	s.fillUserInteraction(posts, currentUserID)
	return posts, total, nil
}

func (s *FeedService) fillUserInteraction(posts []model.Post, userID int64) {
	if userID <= 0 || len(posts) == 0 {
		return
	}
	postIDs := make([]int64, len(posts))
	for i, p := range posts {
		postIDs[i] = p.ID
	}
	likedMap := s.likeRepo.GetUserLikedPostIDs(userID, postIDs)
	for i := range posts {
		posts[i].IsLiked = likedMap[posts[i].ID]
	}
}

type cachedFeedData struct {
	Posts []model.Post `json:"posts"`
	Total int64        `json:"total"`
}

func (s *FeedService) cacheFeed(posts []model.Post, total int64) {
	ctx := context.Background()
	key := "feed:latest:page1"
	data, err := json.Marshal(cachedFeedData{Posts: posts, Total: total})
	if err != nil {
		return
	}
	repository.RDB.Set(ctx, key, data, 2*time.Minute)
}

func (s *FeedService) InvalidateCache() {
	repository.RDB.Del(context.Background(), "feed:latest:page1")
}

func (s *FeedService) getCachedFeed() ([]model.Post, int64, bool) {
	ctx := context.Background()
	key := "feed:latest:page1"
	bytes, err := repository.RDB.Get(ctx, key).Bytes()
	if err != nil {
		return nil, 0, false
	}
	var data cachedFeedData
	if err := json.Unmarshal(bytes, &data); err != nil {
		return nil, 0, false
	}
	fmt.Println("feed served from Redis cache")
	return data.Posts, data.Total, true
}
