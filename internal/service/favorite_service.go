package service

import (
	"context"
	"errors"

	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

var ErrFavoritePostNotFound = errors.New("帖子不存在")

type favoriteRepository interface {
	Ensure(context.Context, int64, int64) error
	Remove(context.Context, int64, int64) error
	ListVisible(context.Context, int64, int, int) ([]model.Post, int64, error)
}

type favoritePostRepository interface {
	FindByIDLight(int64) (*model.Post, error)
}

type FavoriteService struct {
	repo     favoriteRepository
	postRepo favoritePostRepository
}

func NewFavoriteService() *FavoriteService {
	return &FavoriteService{
		repo:     repository.NewFavoriteRepo(),
		postRepo: repository.NewPostRepo(),
	}
}

func (s *FavoriteService) Ensure(ctx context.Context, userID, postID int64) error {
	if userID <= 0 || postID <= 0 {
		return ErrFavoritePostNotFound
	}
	post, err := s.postRepo.FindByIDLight(postID)
	if err != nil || post == nil || post.IsDeleted == 1 || post.Status != model.StatusApproved {
		return ErrFavoritePostNotFound
	}
	return s.repo.Ensure(ctx, userID, postID)
}

func (s *FavoriteService) Remove(ctx context.Context, userID, postID int64) error {
	if userID <= 0 || postID <= 0 {
		return ErrFavoritePostNotFound
	}
	return s.repo.Remove(ctx, userID, postID)
}

func (s *FavoriteService) List(ctx context.Context, userID int64, page, pageSize int) ([]model.Post, int64, int, int, error) {
	page, pageSize = clampPage(page, pageSize)
	posts, total, err := s.repo.ListVisible(ctx, userID, page, pageSize)
	return posts, total, page, pageSize, err
}
