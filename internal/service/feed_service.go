package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

type FeedService struct {
	postRepo *repository.PostRepo
	likeRepo *repository.LikeRepo
	search   *hybridPostSearch
}

func NewFeedService() *FeedService {
	postRepo := repository.NewPostRepo()
	return &FeedService{
		postRepo: postRepo,
		likeRepo: repository.NewLikeRepo(),
		search:   newHybridPostSearch(postRepo),
	}
}

type FeedReq struct {
	Sort     string `form:"sort"` // model.SortLatest (default) or "hot"
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
		req.Sort = model.SortLatest
	}

	// Try Redis cache for first page of latest feed (hot feed always fresh)
	if req.Page == 1 && req.Sort == model.SortLatest && req.UserID == nil {
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
	if req.Page == 1 && req.Sort == model.SortLatest && req.UserID == nil {
		s.cacheFeed(posts, total)
	}

	return posts, total, nil
}

func (s *FeedService) GetFollowingFeed(req FeedReq, currentUserID int64) ([]model.Post, int64, error) {
	if currentUserID <= 0 {
		return nil, 0, errors.New("following feed requires authentication")
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 50 {
		req.PageSize = 20
	}
	posts, total, err := s.postRepo.FollowingFeed(currentUserID, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, err
	}
	s.fillUserInteraction(posts, currentUserID)
	return posts, total, nil
}

func (s *FeedService) Search(ctx context.Context, q string, page, pageSize int, currentUserID int64) ([]model.Post, int64, error) {
	posts, total, err := s.search.Search(ctx, q, page, pageSize)
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

// cachedFeedData stores only post IDs and total count — not full Post objects.
// This reduces Redis memory pressure and ensures user data (avatars, usernames) is always fresh.
type cachedFeedData struct {
	PostIDs []int64 `json:"post_ids"`
	Total   int64   `json:"total"`
}

func (s *FeedService) cacheFeed(posts []model.Post, total int64) {
	if repository.RDB == nil {
		return
	}
	ctx := context.Background()
	key := "feed:latest:page1"
	ids := make([]int64, len(posts))
	for i, p := range posts {
		ids[i] = p.ID
	}
	data, err := json.Marshal(cachedFeedData{PostIDs: ids, Total: total})
	if err != nil {
		return
	}
	repository.RDB.Set(ctx, key, data, 2*time.Minute)
}

func (s *FeedService) InvalidateCache() {
	if repository.RDB == nil {
		return
	}
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
	// Re-query from DB to get fresh data (avatars, usernames, etc.)
	posts, err := s.postRepo.FindByIDs(data.PostIDs)
	if err != nil {
		slog.Warn("failed to re-query cached feed posts", "err", err)
		return nil, 0, false
	}
	slog.Debug("feed served from Redis cache", "cache_format", "ids")
	return posts, data.Total, true
}
