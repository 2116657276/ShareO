package repository

import (
	"context"
	"errors"
	"time"

	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrUploadArtifactInvalid = errors.New("upload artifact invalid")

type UploadArtifactRepo struct {
	db *gorm.DB
}

func NewUploadArtifactRepo(db ...*gorm.DB) *UploadArtifactRepo {
	if len(db) > 0 && db[0] != nil {
		return &UploadArtifactRepo{db: db[0]}
	}
	return &UploadArtifactRepo{db: DB}
}

func (r *UploadArtifactRepo) database() *gorm.DB {
	if r.db != nil {
		return r.db
	}
	return DB
}

func (r *UploadArtifactRepo) Register(ctx context.Context, userID int64, batchID string, objectKeys []string, expiresAt time.Time) error {
	if userID <= 0 || batchID == "" || len(objectKeys) == 0 || expiresAt.IsZero() {
		return ErrUploadArtifactInvalid
	}
	rows := make([]model.UploadArtifact, 0, len(objectKeys))
	seen := make(map[string]struct{}, len(objectKeys))
	for _, objectKey := range objectKeys {
		if objectKey == "" {
			continue
		}
		if _, ok := seen[objectKey]; ok {
			continue
		}
		seen[objectKey] = struct{}{}
		rows = append(rows, model.UploadArtifact{UserID: userID, BatchID: batchID, ObjectKey: objectKey, ExpiresAt: expiresAt})
	}
	if len(rows) == 0 || r.database() == nil {
		return ErrUploadArtifactInvalid
	}
	return r.database().WithContext(ctx).Create(&rows).Error
}

// BindForPost validates submitted URLs belong to the caller and claims every
// generated object in each upload batch in the same transaction as post rows.
func (r *UploadArtifactRepo) BindForPost(ctx context.Context, tx *gorm.DB, userID, postID int64, objectKeys []string) error {
	if userID <= 0 || postID <= 0 || len(objectKeys) == 0 || tx == nil {
		return ErrUploadArtifactInvalid
	}
	var submitted []model.UploadArtifact
	query := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND object_key IN ? AND post_id IS NULL AND expires_at > ?", userID, objectKeys, time.Now())
	if err := query.Find(&submitted).Error; err != nil {
		return err
	}
	if len(submitted) != len(uniqueStrings(objectKeys)) {
		return ErrUploadArtifactInvalid
	}
	batchIDs := make([]string, 0, len(submitted))
	seenBatches := make(map[string]struct{}, len(submitted))
	for _, row := range submitted {
		if _, ok := seenBatches[row.BatchID]; ok {
			continue
		}
		seenBatches[row.BatchID] = struct{}{}
		batchIDs = append(batchIDs, row.BatchID)
	}
	now := time.Now()
	result := tx.WithContext(ctx).Model(&model.UploadArtifact{}).
		Where("user_id = ? AND batch_id IN ? AND post_id IS NULL AND expires_at > ?", userID, batchIDs, now).
		Updates(map[string]interface{}{"post_id": postID, "bound_at": now})
	return result.Error
}

func (r *UploadArtifactRepo) ReleaseForPost(ctx context.Context, tx *gorm.DB, postID int64) error {
	if tx == nil || postID <= 0 {
		return nil
	}
	now := time.Now()
	return tx.WithContext(ctx).Model(&model.UploadArtifact{}).Where("post_id = ?", postID).
		Updates(map[string]interface{}{"post_id": nil, "bound_at": nil, "expires_at": now}).Error
}

// DeleteExpiredUnbound removes metadata first and returns object keys for
// MinIO deletion after the transaction commits.
func (r *UploadArtifactRepo) DeleteExpiredUnbound(ctx context.Context, now time.Time, limit int) ([]string, error) {
	db := r.database()
	if db == nil {
		return nil, ErrUploadArtifactInvalid
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var rows []model.UploadArtifact
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("post_id IS NULL AND expires_at <= ?", now).
			Order("id ASC").Limit(limit).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		ids := make([]int64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		return tx.Where("id IN ?", ids).Delete(&model.UploadArtifact{}).Error
	})
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, row.ObjectKey)
	}
	return keys, nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
