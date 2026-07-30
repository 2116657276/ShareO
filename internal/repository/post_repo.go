package repository

import (
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm"
)

type PostRepo struct{}

func NewPostRepo() *PostRepo { return &PostRepo{} }

type IndexImage struct {
	ImageID   int64     `json:"image_id"`
	ObjectKey string    `json:"object_key"`
	ImageURL  string    `json:"image_url"`
	CreatedAt time.Time `json:"created_at"`
}

type IndexPayload struct {
	PostID    int64        `json:"post_id"`
	Status    string       `json:"status"`
	Content   string       `json:"content"`
	CreatedAt time.Time    `json:"created_at"`
	Images    []IndexImage `json:"images"`
}

type PostSearchCandidate struct {
	PostID    int64     `json:"post_id"`
	Content   string    `json:"content"`
	Score     float64   `json:"score"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *PostRepo) SearchKeywordCandidates(query string, limit int) ([]PostSearchCandidate, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit < 1 || limit > 200 {
		limit = 200
	}
	var candidates []PostSearchCandidate
	if hasFulltext {
		err := DB.Raw(
			`SELECT id AS post_id, content, created_at,
			 MATCH(content) AGAINST(? IN NATURAL LANGUAGE MODE) AS score
			 FROM posts
			 WHERE status = ? AND is_deleted = 0
			   AND MATCH(content) AGAINST(? IN NATURAL LANGUAGE MODE)
			 ORDER BY score DESC, id DESC LIMIT ?`,
			query, model.StatusApproved, query, limit,
		).Scan(&candidates).Error
		if err == nil {
			return candidates, nil
		}
		slog.Warn("keyword candidate full-text search failed; falling back to LIKE", "err", err)
	}
	err := DB.Model(&model.Post{}).
		Select("id AS post_id, content, created_at, 0 AS score").
		Where("status = ? AND is_deleted = 0 AND content LIKE ?",
			model.StatusApproved, "%"+escapeLikePattern(query)+"%").
		Order("created_at DESC, id DESC").
		Limit(limit).
		Scan(&candidates).Error
	return candidates, err
}

func (r *PostRepo) SearchApprovedPosts(query string, limit int) ([]IndexPayload, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit < 1 || limit > 10 {
		limit = 10
	}
	var posts []model.Post
	db := DB.Where("status = ? AND is_deleted = 0", model.StatusApproved)
	if hasFulltext {
		db = db.Where("MATCH(content) AGAINST(? IN NATURAL LANGUAGE MODE)", query).
			Order(gorm.Expr("MATCH(content) AGAINST(? IN NATURAL LANGUAGE MODE) DESC, id DESC", query))
	} else {
		db = db.Where("content LIKE ?", "%"+query+"%").Order("id DESC")
	}
	if err := db.Preload("Images", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Limit(limit).Find(&posts).Error; err != nil {
		return nil, err
	}
	items := make([]IndexPayload, 0, len(posts))
	for _, post := range posts {
		items = append(items, buildIndexPayload(post))
	}
	return items, nil
}

func (r *PostRepo) ReadApprovedPosts(ids []int64) ([]IndexPayload, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > 10 {
		ids = ids[:10]
	}
	posts, err := r.FindByIDs(ids)
	if err != nil {
		return nil, err
	}
	items := make([]IndexPayload, 0, len(posts))
	for index := range posts {
		items = append(items, buildIndexPayload(posts[index]))
	}
	return items, nil
}

// hasFulltext is set after DB init based on whether the FULLTEXT index is available.
var hasFulltext bool

// SetDB allows tests to replace the global DB instance (for test isolation).
func SetDB(db *gorm.DB) { DB = db }

// DetectFulltext checks if FULLTEXT index is available on the posts table.
// Called from InitDB after DB connection is established.
// Uses INFORMATION_SCHEMA instead of MATCH...AGAINST to avoid false negatives on empty tables.
func DetectFulltext() {
	if DB != nil {
		var count int64
		err := DB.Raw(
			"SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'posts' AND INDEX_TYPE = 'FULLTEXT'",
		).Scan(&count).Error
		hasFulltext = err == nil && count > 0
	}
}

func (r *PostRepo) Create(post *model.Post) error {
	return DB.Create(post).Error
}

func (r *PostRepo) FindByID(id int64) (*model.Post, error) {
	var post model.Post
	err := DB.Preload("User").Preload("Images", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).First(&post, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &post, err
}

// FindByIDs fetches multiple posts by their IDs with full Preloads, preserving the input order.
func (r *PostRepo) FindByIDs(ids []int64) ([]model.Post, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var posts []model.Post
	err := DB.Where("id IN ? AND is_deleted = 0 AND status = ?", ids, model.StatusApproved).
		Preload("User").Preload("Images", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Find(&posts).Error
	if err != nil {
		return nil, err
	}
	// Preserve the original order from ids
	orderMap := make(map[int64]int, len(ids))
	for i, id := range ids {
		orderMap[id] = i
	}
	sort.Slice(posts, func(i, j int) bool {
		return orderMap[posts[i].ID] < orderMap[posts[j].ID]
	})
	return posts, nil
}

// FindByIDLight fetches only the post record without Preloads, for permission checks.
func (r *PostRepo) FindByIDLight(id int64) (*model.Post, error) {
	var post model.Post
	err := DB.First(&post, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &post, err
}

// GetIndexPayload returns only approved, non-deleted post data required by the
// AI worker. Image bytes stay behind Go's authenticated storage proxy; the
// worker never receives MinIO credentials.
func (r *PostRepo) GetIndexPayload(id int64) (*IndexPayload, error) {
	var post model.Post
	err := DB.Where("id = ? AND status = ? AND is_deleted = 0", id, model.StatusApproved).
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC") }).First(&post).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	payload := buildIndexPayload(post)
	return &payload, nil
}

func (r *PostRepo) ListIndexPayloads(afterID int64, limit int) ([]IndexPayload, error) {
	var posts []model.Post
	err := DB.Where("id > ? AND status = ? AND is_deleted = 0", afterID, model.StatusApproved).
		Order("id ASC").Limit(limit).
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC") }).Find(&posts).Error
	if err != nil {
		return nil, err
	}
	payloads := make([]IndexPayload, 0, len(posts))
	for _, post := range posts {
		payloads = append(payloads, buildIndexPayload(post))
	}
	return payloads, nil
}

func buildIndexPayload(post model.Post) IndexPayload {
	payload := IndexPayload{PostID: post.ID, Status: post.Status, Content: post.Content, CreatedAt: post.CreatedAt}
	for _, image := range post.Images {
		objectKey := image.ImageURL
		if marker := strings.Index(objectKey, "/api/v1/images/"); marker >= 0 {
			objectKey = objectKey[marker+len("/api/v1/images/"):]
		}
		objectKey = strings.TrimPrefix(objectKey, "/")
		if objectKey == "" {
			continue
		}
		payload.Images = append(payload.Images, IndexImage{
			ImageID: image.ID, ObjectKey: objectKey,
			ImageURL: "/api/v1/images/" + objectKey, CreatedAt: image.CreatedAt,
		})
	}
	return payload
}

func (r *PostRepo) Update(post *model.Post) error {
	// Use Updates with specific fields to avoid cascade-saving Preloaded associations
	// (User, Images, etc.)
	return DB.Model(post).Updates(map[string]interface{}{
		"content": post.Content,
		"status":  post.Status,
	}).Error
}

func (r *PostRepo) SoftDelete(id, userID int64) error {
	return DB.Model(&model.Post{}).Where("id = ? AND user_id = ?", id, userID).
		Update("is_deleted", 1).Error
}

// FeedQuery holds all possible feed query parameters
type FeedQuery struct {
	UserID   *int64 // filter by author
	Status   string // post status filter
	Sort     string // "latest" or "hot"
	Page     int
	PageSize int
}

func (r *PostRepo) Feed(q FeedQuery) ([]model.Post, int64, error) {
	var posts []model.Post
	var total int64

	query := DB.Model(&model.Post{}).Where("is_deleted = 0")

	if q.UserID != nil {
		query = query.Where("user_id = ?", *q.UserID)
	}
	if q.Status != "" {
		query = query.Where("status = ?", q.Status)
	} else {
		query = query.Where("status = ?", model.StatusApproved)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderClause := "created_at DESC"
	if q.Sort == model.SortHot {
		// Weighted hot score: likes(3) + comments(2) + views(1)
		orderClause = "(like_count * 3 + comment_count * 2 + view_count) DESC, created_at DESC"
	}

	offset := (q.Page - 1) * q.PageSize
	err := query.Preload("User").Preload("Images", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Order(orderClause).Offset(offset).Limit(q.PageSize).Find(&posts).Error

	return posts, total, err
}

// FollowingFeed returns approved posts authored by users followed by userID.
// The follows join keeps filtering and pagination in MySQL instead of merging
// one request per followed user in the browser.
func (r *PostRepo) FollowingFeed(userID int64, page, pageSize int) ([]model.Post, int64, error) {
	var posts []model.Post
	var total int64
	query := DB.Model(&model.Post{}).
		Joins("JOIN follows ON follows.followee_id = posts.user_id AND follows.follower_id = ?", userID).
		Where("posts.is_deleted = 0 AND posts.status = ?", model.StatusApproved)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if err := query.Preload("User").Preload("Images", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Order("posts.created_at DESC, posts.id DESC").Offset(offset).Limit(pageSize).Find(&posts).Error; err != nil {
		return nil, 0, err
	}
	return posts, total, nil
}

func (r *PostRepo) IncrementView(id int64) {
	if err := DB.Model(&model.Post{}).Where("id = ?", id).UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error; err != nil {
		slog.Warn("failed to increment post view count", "post_id", id, "err", err)
	}
}

func (r *PostRepo) UpdateStatus(id int64, status, comment string, reviewerID int64) error {
	updates := map[string]interface{}{
		"status":      status,
		"reviewed_by": reviewerID,
		"reviewed_at": time.Now(),
	}
	// Only update review_comment if provided (preserve previous rejection reason on approve)
	if comment != "" {
		updates["review_comment"] = comment
	}
	return DB.Model(&model.Post{}).Where("id = ?", id).Updates(updates).Error
}

func (r *PostRepo) AdminSoftDelete(id int64) error {
	return DB.Model(&model.Post{}).Where("id = ?", id).Update("is_deleted", 1).Error
}

func (r *PostRepo) CountByStatus(status string) int64 {
	var count int64
	if err := DB.Model(&model.Post{}).Where("status = ? AND is_deleted = 0", status).Count(&count).Error; err != nil {
		slog.Warn("failed to count posts by status", "status", status, "err", err)
	}
	return count
}

func (r *PostRepo) CountTotal() int64 {
	var count int64
	if err := DB.Model(&model.Post{}).Where("is_deleted = 0").Count(&count).Error; err != nil {
		slog.Warn("failed to count posts", "err", err)
	}
	return count
}

func (r *PostRepo) CountByUser(userID int64) int64 {
	var count int64
	if err := DB.Model(&model.Post{}).Where("user_id = ? AND is_deleted = 0 AND status = ?", userID, model.StatusApproved).Count(&count).Error; err != nil {
		slog.Warn("failed to count posts by user", "user_id", userID, "err", err)
	}
	return count
}

// sanitizeSearchQuery removes BOOLEAN mode operators from user input to prevent
// MySQL FULLTEXT syntax errors. The characters removed are: + - ~ < > ( ) * " @
func sanitizeSearchQuery(q string) string {
	replacer := strings.NewReplacer(
		"+", " ", "-", " ", "~", " ", "<", " ", ">", " ",
		"(", " ", ")", " ", "*", " ", "\"", " ", "@", " ",
	)
	return replacer.Replace(q)
}

// escapeLikePattern escapes LIKE special characters % and _.
func escapeLikePattern(q string) string {
	q = strings.NewReplacer("%", "\\%", "_", "\\_").Replace(q)
	return q
}

func (r *PostRepo) Search(q string, page, pageSize int) ([]model.Post, int64, error) {
	var posts []model.Post
	var total int64

	base := DB.Model(&model.Post{}).Where("is_deleted = 0 AND status = ?", model.StatusApproved)

	if hasFulltext {
		// Sanitize BOOLEAN operators to prevent syntax errors
		clean := sanitizeSearchQuery(q)
		base = base.Where("MATCH(content) AGAINST(? IN BOOLEAN MODE)", clean)
		if err := base.Count(&total).Error; err != nil {
			// FULLTEXT failed — fall back to LIKE with escaped pattern
			slog.Warn("full-text post search failed; falling back to LIKE", "err", err)
			base = DB.Model(&model.Post{}).Where("is_deleted = 0 AND status = ?", model.StatusApproved)
			base = base.Where("content LIKE ?", "%"+escapeLikePattern(q)+"%")
			if countErr := base.Count(&total).Error; countErr != nil {
				return nil, 0, countErr
			}
		}
	} else {
		base = base.Where("content LIKE ?", "%"+escapeLikePattern(q)+"%")
		if err := base.Count(&total).Error; err != nil {
			return nil, 0, err
		}
	}

	offset := (page - 1) * pageSize
	err := base.Preload("User").Preload("Images", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&posts).Error

	return posts, total, err
}
