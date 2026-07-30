package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

const postSearchCandidateLimit = 200

var ErrPostSearchUnavailable = errors.New("post search unavailable")

const (
	defaultPostSemanticThreshold = 0.54
	minKeywordLexicalOverlap     = 0.20
)

type semanticPostCandidate struct {
	PostID int64   `json:"post_id"`
	Score  float64 `json:"score"`
}

type semanticPostResponse struct {
	Results []semanticPostCandidate `json:"results"`
}

type postSearchRepository interface {
	SearchKeywordCandidates(string, int) ([]repository.PostSearchCandidate, error)
	FindByIDs([]int64) ([]model.Post, error)
}

type semanticPostSearcher interface {
	Search(context.Context, string, int) ([]semanticPostCandidate, error)
}

type httpSemanticPostSearcher struct {
	client    *http.Client
	aiBaseURL string
	token     string
}

func newHTTPSemanticPostSearcher() *httpSemanticPostSearcher {
	baseURL := strings.TrimRight(os.Getenv("SHAREO_AI_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8000"
	}
	return &httpSemanticPostSearcher{
		client:    &http.Client{Timeout: 8 * time.Second},
		aiBaseURL: baseURL,
		token:     os.Getenv("SHAREO_INTERNAL_TOKEN"),
	}
}

func (s *httpSemanticPostSearcher) Search(ctx context.Context, query string, limit int) ([]semanticPostCandidate, error) {
	body, err := json.Marshal(map[string]any{"query": query, "limit": limit})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.aiBaseURL+"/v1/search/posts", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.token != "" {
		req.Header.Set("X-Internal-Token", s.token)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("semantic post search unavailable")
	}
	var payload semanticPostResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Results, nil
}

type hybridPostSearch struct {
	repo              postSearchRepository
	semantic          semanticPostSearcher
	semanticThreshold float64
}

func newHybridPostSearch(repo postSearchRepository) *hybridPostSearch {
	threshold := defaultPostSemanticThreshold
	if raw := strings.TrimSpace(os.Getenv("SHAREO_POST_SEARCH_SEMANTIC_THRESHOLD")); raw != "" {
		if value, err := strconv.ParseFloat(raw, 64); err == nil && value >= 0 && value <= 1 {
			threshold = value
		}
	}
	return &hybridPostSearch{
		repo: repo, semantic: newHTTPSemanticPostSearcher(), semanticThreshold: threshold,
	}
}

type rankedPost struct {
	post  model.Post
	score float64
}

func (s *hybridPostSearch) Search(ctx context.Context, query string, page, pageSize int) ([]model.Post, int64, error) {
	keyword, keywordErr := s.repo.SearchKeywordCandidates(query, postSearchCandidateLimit)
	semantic, semanticErr := s.semantic.Search(ctx, query, postSearchCandidateLimit)
	if keywordErr != nil && semanticErr != nil {
		return nil, 0, ErrPostSearchUnavailable
	}

	keywordRanks := make(map[int64]int, len(keyword))
	semanticRanks := make(map[int64]int, len(semantic))
	ids := make([]int64, 0, len(keyword)+len(semantic))
	seen := make(map[int64]struct{}, len(keyword)+len(semantic))
	for index, item := range keyword {
		if item.PostID <= 0 || !keywordCandidateAccepted(query, item.Content) {
			continue
		}
		if _, ok := keywordRanks[item.PostID]; !ok {
			keywordRanks[item.PostID] = index + 1
		}
		if _, ok := seen[item.PostID]; !ok {
			seen[item.PostID] = struct{}{}
			ids = append(ids, item.PostID)
		}
	}
	for index, item := range semantic {
		if item.PostID <= 0 || item.Score < s.semanticThreshold {
			continue
		}
		if _, ok := semanticRanks[item.PostID]; !ok {
			semanticRanks[item.PostID] = index + 1
		}
		if _, ok := seen[item.PostID]; !ok {
			seen[item.PostID] = struct{}{}
			ids = append(ids, item.PostID)
		}
	}
	posts, err := s.repo.FindByIDs(ids)
	if err != nil {
		return nil, 0, err
	}
	ranked := make([]rankedPost, 0, len(posts))
	for _, post := range posts {
		semanticScore := rankScore(semanticRanks[post.ID])
		keywordScore := rankScore(keywordRanks[post.ID])
		overlap := lexicalOverlap(query, post.Content)
		exact := 0.0
		normalizedQuery := normalizeSearchText(query)
		if normalizedQuery != "" && strings.Contains(normalizeSearchText(post.Content), normalizedQuery) {
			exact = 1
		}
		ranked = append(ranked, rankedPost{
			post:  post,
			score: 0.60*semanticScore + 0.20*keywordScore + 0.15*overlap + 0.05*exact,
		})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if math.Abs(ranked[i].score-ranked[j].score) > 1e-12 {
			return ranked[i].score > ranked[j].score
		}
		if !ranked[i].post.CreatedAt.Equal(ranked[j].post.CreatedAt) {
			return ranked[i].post.CreatedAt.After(ranked[j].post.CreatedAt)
		}
		return ranked[i].post.ID > ranked[j].post.ID
	})
	total := int64(len(ranked))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 12
	}
	start := (page - 1) * pageSize
	if start >= len(ranked) {
		return []model.Post{}, total, nil
	}
	end := min(start+pageSize, len(ranked))
	result := make([]model.Post, 0, end-start)
	for _, item := range ranked[start:end] {
		result = append(result, item.post)
	}
	return result, total, nil
}

func keywordCandidateAccepted(query, content string) bool {
	normalizedQuery := normalizeSearchText(query)
	normalizedContent := normalizeSearchText(content)
	if normalizedQuery == "" || normalizedContent == "" {
		return false
	}
	return strings.Contains(normalizedContent, normalizedQuery) ||
		lexicalOverlap(normalizedQuery, normalizedContent) >= minKeywordLexicalOverlap
}

func rankScore(rank int) float64 {
	if rank <= 0 {
		return 0
	}
	return 1 / math.Log2(float64(rank)+1)
}

func normalizeSearchText(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func searchNgrams(value string) []string {
	runes := []rune(normalizeSearchText(value))
	if len(runes) == 0 {
		return nil
	}
	if len(runes) == 1 {
		return []string{string(runes)}
	}
	result := make([]string, 0, len(runes)-1)
	for index := 0; index < len(runes)-1; index++ {
		result = append(result, string(runes[index:index+2]))
	}
	return result
}

func lexicalOverlap(query, content string) float64 {
	queryTerms := searchNgrams(query)
	contentTerms := searchNgrams(content)
	if len(queryTerms) == 0 || len(contentTerms) == 0 {
		return 0
	}
	queryCount := make(map[string]int, len(queryTerms))
	contentCount := make(map[string]int, len(contentTerms))
	for _, term := range queryTerms {
		queryCount[term]++
	}
	for _, term := range contentTerms {
		contentCount[term]++
	}
	intersection := 0
	for term, count := range queryCount {
		intersection += min(count, contentCount[term])
	}
	return 2 * float64(intersection) / float64(len(queryTerms)+len(contentTerms))
}
