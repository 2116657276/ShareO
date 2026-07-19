package repository

import (
	"errors"
	"log"
	"sort"
	"time"

	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm"
)

type PostRepo struct{}

func NewPostRepo() *PostRepo { return &PostRepo{} }

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
	}).Preload("Topics").Preload("RepostOf").Preload("RepostOf.User").Preload("RepostOf.Images", func(db *gorm.DB) *gorm.DB {
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
	}).Preload("Topics").
		Preload("RepostOf").Preload("RepostOf.User").Preload("RepostOf.Images", func(db *gorm.DB) *gorm.DB {
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

func (r *PostRepo) Update(post *model.Post) error {
	// Use Updates with specific fields to avoid cascade-saving Preloaded associations
	// (User, Images, Topics, RepostOf, etc.)
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
	UserID   *int64  // filter by author
	Status   string  // post status filter
	TopicID  *int64  // filter by topic
	Sort     string  // "latest" or "hot"
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
	if q.TopicID != nil {
		query = query.Joins("JOIN topic_posts ON topic_posts.post_id = posts.id").
			Where("topic_posts.topic_id = ?", *q.TopicID)
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
	}).Preload("Topics").Preload("RepostOf").Preload("RepostOf.User").Preload("RepostOf.Images", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Order(orderClause).Offset(offset).Limit(q.PageSize).Find(&posts).Error

	return posts, total, err
}

func (r *PostRepo) IncrementView(id int64) {
	if err := DB.Model(&model.Post{}).Where("id = ?", id).UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error; err != nil {
		log.Printf("PostRepo.IncrementView(%d): %v", id, err)
	}
}

func (r *PostRepo) IncrementShare(id int64) {
	if err := DB.Model(&model.Post{}).Where("id = ?", id).UpdateColumn("share_count", gorm.Expr("share_count + 1")).Error; err != nil {
		log.Printf("PostRepo.IncrementShare(%d): %v", id, err)
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
		log.Printf("PostRepo.CountByStatus(%s): %v", status, err)
	}
	return count
}

func (r *PostRepo) CountTotal() int64 {
	var count int64
	if err := DB.Model(&model.Post{}).Where("is_deleted = 0").Count(&count).Error; err != nil {
		log.Printf("PostRepo.CountTotal: %v", err)
	}
	return count
}

func (r *PostRepo) CountByUser(userID int64) int64 {
	var count int64
	if err := DB.Model(&model.Post{}).Where("user_id = ? AND is_deleted = 0 AND status = ?", userID, model.StatusApproved).Count(&count).Error; err != nil {
		log.Printf("PostRepo.CountByUser(%d): %v", userID, err)
	}
	return count
}

func (r *PostRepo) Search(q string, page, pageSize int) ([]model.Post, int64, error) {
	var posts []model.Post
	var total int64

	base := DB.Model(&model.Post{}).Where("is_deleted = 0 AND status = ?", model.StatusApproved)

	// Use LIKE search. FULLTEXT requires MySQL ngram index migration to be applied.
	// When the index is available, hasFulltext will be true and FULLTEXT will be used instead.
	if hasFulltext {
		base = base.Where("MATCH(content) AGAINST(? IN BOOLEAN MODE)", q)
	} else {
		base = base.Where("content LIKE ?", "%"+q+"%")
	}

	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := base.Preload("User").Preload("Images", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&posts).Error

	return posts, total, err
}
