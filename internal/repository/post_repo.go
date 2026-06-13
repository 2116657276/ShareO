package repository

import (
	"errors"

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
func DetectFulltext() {
	if DB != nil {
		// GORM Raw() is lazy — must execute via Scan before checking Error.
		var ok int
		err := DB.Raw(
			"SELECT 1 FROM posts WHERE MATCH(content) AGAINST('test' IN BOOLEAN MODE) LIMIT 1",
		).Scan(&ok).Error
		hasFulltext = err == nil
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
	return DB.Save(post).Error
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
	DB.Model(&model.Post{}).Where("id = ?", id).UpdateColumn("view_count", gorm.Expr("view_count + 1"))
}

func (r *PostRepo) IncrementShare(id int64) {
	DB.Model(&model.Post{}).Where("id = ?", id).UpdateColumn("share_count", gorm.Expr("share_count + 1"))
}

func (r *PostRepo) UpdateStatus(id int64, status, comment string, reviewerID int64) error {
	updates := map[string]interface{}{
		"status":         status,
		"review_comment": comment,
		"reviewed_by":    reviewerID,
	}
	return DB.Model(&model.Post{}).Where("id = ?", id).Updates(updates).Error
}

func (r *PostRepo) AdminSoftDelete(id int64) error {
	return DB.Model(&model.Post{}).Where("id = ?", id).Update("is_deleted", 1).Error
}

func (r *PostRepo) CountByStatus(status string) int64 {
	var count int64
	DB.Model(&model.Post{}).Where("status = ? AND is_deleted = 0", status).Count(&count)
	return count
}

func (r *PostRepo) CountTotal() int64 {
	var count int64
	DB.Model(&model.Post{}).Where("is_deleted = 0").Count(&count)
	return count
}

func (r *PostRepo) CountByUser(userID int64) int64 {
	var count int64
	DB.Model(&model.Post{}).Where("user_id = ? AND is_deleted = 0 AND status = ?", userID, model.StatusApproved).Count(&count)
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
