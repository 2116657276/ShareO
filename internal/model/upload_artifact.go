package model

import "time"

// UploadArtifact tracks generated MinIO objects until a post claims them.
// Bound objects remain associated with their post; unbound objects are
// eligible for expiry cleanup.
type UploadArtifact struct {
	ID        int64      `gorm:"primaryKey;autoIncrement" json:"-"`
	UserID    int64      `gorm:"not null;index:idx_upload_artifacts_user_batch,priority:1" json:"-"`
	BatchID   string     `gorm:"type:uuid;not null;index:idx_upload_artifacts_user_batch,priority:2" json:"-"`
	ObjectKey string     `gorm:"type:varchar(500);not null;uniqueIndex" json:"-"`
	PostID    *int64     `gorm:"index:idx_upload_artifacts_post" json:"-"`
	ExpiresAt time.Time  `gorm:"not null;index:idx_upload_artifacts_expiry" json:"-"`
	BoundAt   *time.Time `json:"-"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"-"`
}

func (UploadArtifact) TableName() string { return "upload_artifacts" }
