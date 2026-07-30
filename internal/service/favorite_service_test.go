package service

import (
	"context"
	"errors"
	"testing"

	"github.com/zhoujianlin/ShareO/internal/model"
)

type fakeFavoriteRepo struct {
	ensureCalls int
	removeCalls int
	posts       []model.Post
	total       int64
	err         error
}

func (f *fakeFavoriteRepo) Ensure(context.Context, int64, int64) error {
	f.ensureCalls++
	return f.err
}

func (f *fakeFavoriteRepo) Remove(context.Context, int64, int64) error {
	f.removeCalls++
	return f.err
}

func (f *fakeFavoriteRepo) ListVisible(context.Context, int64, int, int) ([]model.Post, int64, error) {
	return f.posts, f.total, f.err
}

type fakeFavoritePostRepo struct {
	post *model.Post
	err  error
}

func (f *fakeFavoritePostRepo) FindByIDLight(int64) (*model.Post, error) {
	return f.post, f.err
}

func TestFavoriteEnsureRequiresVisiblePost(t *testing.T) {
	repo := &fakeFavoriteRepo{}
	service := &FavoriteService{
		repo: repo,
		postRepo: &fakeFavoritePostRepo{post: &model.Post{
			ID: 7, Status: model.StatusPending,
		}},
	}
	if err := service.Ensure(context.Background(), 2, 7); !errors.Is(err, ErrFavoritePostNotFound) {
		t.Fatalf("err = %v, want ErrFavoritePostNotFound", err)
	}
	if repo.ensureCalls != 0 {
		t.Fatalf("ensure calls = %d, want 0", repo.ensureCalls)
	}
}

func TestFavoriteEnsureAndRemoveAreDelegated(t *testing.T) {
	repo := &fakeFavoriteRepo{}
	service := &FavoriteService{
		repo: repo,
		postRepo: &fakeFavoritePostRepo{post: &model.Post{
			ID: 7, Status: model.StatusApproved,
		}},
	}
	if err := service.Ensure(context.Background(), 2, 7); err != nil {
		t.Fatal(err)
	}
	if err := service.Ensure(context.Background(), 2, 7); err != nil {
		t.Fatal(err)
	}
	if err := service.Remove(context.Background(), 2, 7); err != nil {
		t.Fatal(err)
	}
	if err := service.Remove(context.Background(), 2, 7); err != nil {
		t.Fatal(err)
	}
	if repo.ensureCalls != 2 || repo.removeCalls != 2 {
		t.Fatalf("ensure=%d remove=%d", repo.ensureCalls, repo.removeCalls)
	}
}

func TestFavoriteListClampsPagination(t *testing.T) {
	repo := &fakeFavoriteRepo{posts: []model.Post{{ID: 1}}, total: 1}
	service := &FavoriteService{repo: repo, postRepo: &fakeFavoritePostRepo{}}
	posts, total, page, pageSize, err := service.List(context.Background(), 3, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 || total != 1 || page != 1 || pageSize != 20 {
		t.Fatalf("posts=%d total=%d page=%d pageSize=%d", len(posts), total, page, pageSize)
	}
}
