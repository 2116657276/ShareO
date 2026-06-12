package model

import "time"

type Post struct {
	ID            int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID        int64      `gorm:"not null;index:idx_posts_user,priority:1" json:"user_id"`
	Content       string     `gorm:"type:text" json:"content"`
	CoverImage    string     `gorm:"type:varchar(500);default:''" json:"cover_image"`
	ViewCount     int        `gorm:"default:0" json:"view_count"`
	LikeCount     int        `gorm:"default:0" json:"like_count"`
	CommentCount  int        `gorm:"default:0" json:"comment_count"`
	FavoriteCount int        `gorm:"default:0" json:"favorite_count"`
	ShareCount    int        `gorm:"default:0" json:"share_count"`
	IsRepost      int8       `gorm:"default:0" json:"is_repost"`
	RepostOfID    *int64     `json:"repost_of_id"`
	RepostText    string     `gorm:"type:text" json:"repost_text"`
	Status        string     `gorm:"type:enum('pending','approved','rejected');default:'pending';index:idx_posts_status,priority:1" json:"status"`
	ReviewComment string     `gorm:"type:varchar(500);default:''" json:"review_comment"`
	ReviewedBy    *int64     `json:"reviewed_by"`
	ReviewedAt    *time.Time `json:"reviewed_at"`
	IsDeleted     int8       `gorm:"default:0;index:idx_posts_deleted" json:"is_deleted"`
	CreatedAt     time.Time  `gorm:"autoCreateTime;index:idx_posts_created,priority:1,sort:desc" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime" json:"updated_at"`

	User        *User       `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Images      []PostImage `gorm:"foreignKey:PostID" json:"images,omitempty"`
	Reviewer    *User       `gorm:"foreignKey:ReviewedBy" json:"reviewer,omitempty"`
	Topics      []Topic     `gorm:"many2many:topic_posts" json:"topics,omitempty"`
	RepostOf    *Post       `gorm:"foreignKey:RepostOfID" json:"repost_of,omitempty"`
	IsLiked     bool        `gorm:"-" json:"is_liked"`
	IsFavorited bool        `gorm:"-" json:"is_favorited"`
}

func (Post) TableName() string { return "posts" }
