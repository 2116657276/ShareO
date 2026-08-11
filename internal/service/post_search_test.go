package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

type fakePostSearchRepo struct {
	keyword    []repository.PostSearchCandidate
	keywordErr error
	posts      []model.Post
	findIDs    []int64
}

func (f *fakePostSearchRepo) SearchKeywordCandidates(string, int) ([]repository.PostSearchCandidate, error) {
	return f.keyword, f.keywordErr
}

func (f *fakePostSearchRepo) FindByIDs(ids []int64) ([]model.Post, error) {
	f.findIDs = append([]int64(nil), ids...)
	allowed := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		allowed[id] = struct{}{}
	}
	posts := make([]model.Post, 0, len(ids))
	for _, post := range f.posts {
		if _, ok := allowed[post.ID]; ok {
			posts = append(posts, post)
		}
	}
	return posts, nil
}

type fakeSemanticPostSearch struct {
	items []semanticPostCandidate
	err   error
}

func (f *fakeSemanticPostSearch) Search(context.Context, string, int) ([]semanticPostCandidate, error) {
	return f.items, f.err
}

func TestLexicalOverlapUsesChineseBigrams(t *testing.T) {
	if got := normalizeSearchText(" 西湖，夜景! "); got != "西湖夜景" {
		t.Fatalf("normalizeSearchText() = %q", got)
	}
	if got := lexicalOverlap("西湖夜景", "今晚的西湖夜景真美"); got <= 0.4 {
		t.Fatalf("expected meaningful overlap, got %.4f", got)
	}
	if got := lexicalOverlap("山", "远山"); got != 0 {
		t.Fatalf("single-character unigram should not match unrelated bigrams, got %.4f", got)
	}
}

func TestRankScore(t *testing.T) {
	if got := rankScore(1); got != 1 {
		t.Fatalf("rankScore(1) = %.4f", got)
	}
	if rankScore(2) >= rankScore(1) || rankScore(0) != 0 {
		t.Fatal("rank scores must decrease and missing ranks must be zero")
	}
}

func TestHybridPostSearchSemanticFirstWithLexicalBonus(t *testing.T) {
	now := time.Now()
	repo := &fakePostSearchRepo{
		keyword: []repository.PostSearchCandidate{
			{PostID: 2, Content: "西湖夜景", CreatedAt: now},
			{PostID: 1, Content: "湖边灯光", CreatedAt: now.Add(-time.Minute)},
		},
		posts: []model.Post{
			{ID: 1, Content: "夜晚湖边灯光倒映", CreatedAt: now.Add(-time.Minute)},
			{ID: 2, Content: "西湖夜景", CreatedAt: now},
			{ID: 3, Content: "城市夜色", CreatedAt: now.Add(-2 * time.Minute)},
		},
	}
	search := &hybridPostSearch{
		repo:              repo,
		semanticThreshold: 0,
		semantic: &fakeSemanticPostSearch{items: []semanticPostCandidate{
			{PostID: 1, Score: 0.93},
			{PostID: 3, Score: 0.90},
			{PostID: 2, Score: 0.85},
		}},
	}
	posts, total, err := search.Search(context.Background(), "西湖夜景", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(posts) != 3 {
		t.Fatalf("total=%d len=%d", total, len(posts))
	}
	if posts[0].ID != 2 || posts[1].ID != 1 {
		t.Fatalf("exact overlap should boost a strong semantic result, order=%v", []int64{posts[0].ID, posts[1].ID, posts[2].ID})
	}
}

func TestHybridPostSearchFallsBackToKeyword(t *testing.T) {
	repo := &fakePostSearchRepo{
		keyword: []repository.PostSearchCandidate{{PostID: 4, Content: "春日桃花"}},
		posts:   []model.Post{{ID: 4, Content: "春日桃花"}},
	}
	search := &hybridPostSearch{
		repo: repo, semanticThreshold: 0,
		semantic: &fakeSemanticPostSearch{err: errors.New("pgvector unavailable")},
	}
	posts, total, err := search.Search(context.Background(), "桃花", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(posts) != 1 || posts[0].ID != 4 {
		t.Fatalf("unexpected fallback result: total=%d posts=%+v", total, posts)
	}
}

func TestHybridPostSearchUsesSemanticWhenKeywordFails(t *testing.T) {
	repo := &fakePostSearchRepo{
		keywordErr: errors.New("postgres unavailable"),
		posts:      []model.Post{{ID: 5, Content: "安静的校园角落"}},
	}
	search := &hybridPostSearch{
		repo: repo, semanticThreshold: 0,
		semantic: &fakeSemanticPostSearch{items: []semanticPostCandidate{{PostID: 5, Score: 0.9}}},
	}
	posts, total, err := search.Search(context.Background(), "校园里安静的地方", 1, 10)
	if err != nil || total != 1 || len(posts) != 1 {
		t.Fatalf("err=%v total=%d posts=%+v", err, total, posts)
	}
}

func TestHybridPostSearchFailsWhenBothChannelsFail(t *testing.T) {
	search := &hybridPostSearch{
		repo:     &fakePostSearchRepo{keywordErr: errors.New("postgres unavailable")},
		semantic: &fakeSemanticPostSearch{err: errors.New("pgvector unavailable")},
	}
	if _, _, err := search.Search(context.Background(), "测试", 1, 10); err == nil {
		t.Fatal("expected all-channel failure")
	}
}

func TestHybridPostSearchPaginationAndStableTieBreak(t *testing.T) {
	now := time.Now()
	repo := &fakePostSearchRepo{
		keyword: []repository.PostSearchCandidate{{PostID: 1}, {PostID: 2}},
		posts: []model.Post{
			{ID: 1, Content: "无重合", CreatedAt: now},
			{ID: 2, Content: "无重合", CreatedAt: now},
		},
	}
	search := &hybridPostSearch{
		repo: repo, semanticThreshold: 0,
		semantic: &fakeSemanticPostSearch{items: []semanticPostCandidate{{PostID: 1}, {PostID: 2}}},
	}
	posts, total, err := search.Search(context.Background(), "测试", 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(posts) != 1 || posts[0].ID != 2 {
		t.Fatalf("unexpected page: total=%d posts=%+v", total, posts)
	}
}

func TestHybridPostSearchFiltersWeakNoMatchCandidates(t *testing.T) {
	repo := &fakePostSearchRepo{
		keyword: []repository.PostSearchCandidate{
			{PostID: 7, Content: "海面上的飞鸟好自由"},
		},
		posts: []model.Post{{ID: 7, Content: "海面上的飞鸟好自由"}},
	}
	search := &hybridPostSearch{
		repo: repo, semanticThreshold: defaultPostSemanticThreshold,
		semantic: &fakeSemanticPostSearch{
			items: []semanticPostCandidate{{PostID: 7, Score: 0.37}},
		},
	}
	posts, total, err := search.Search(context.Background(), "月球上的宇航员", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(posts) != 0 {
		t.Fatalf("weak keyword and semantic candidates must be filtered: total=%d posts=%+v", total, posts)
	}
}
