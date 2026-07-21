package repository

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm"
)

type TopicRepo struct{}

func NewTopicRepo() *TopicRepo { return &TopicRepo{} }

func (r *TopicRepo) CountByStatus(status int8) int64 {
	var count int64
	if err := DB.Model(&model.Topic{}).Where("status = ?", status).Count(&count).Error; err != nil {
		slog.Warn("failed to count topics by status", "status", status, "err", err)
	}
	return count
}

func (r *TopicRepo) Create(topic *model.Topic) error {
	return DB.Create(topic).Error
}

func (r *TopicRepo) FindByID(id int64) (*model.Topic, error) {
	var topic model.Topic
	err := DB.First(&topic, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &topic, err
}

func (r *TopicRepo) List(onlyActive bool) ([]model.Topic, error) {
	var topics []model.Topic
	query := DB.Model(&model.Topic{})
	if onlyActive {
		query = query.Where("status = ?", model.TopicStatusActive)
	}
	err := query.Order("post_count DESC, created_at DESC").Find(&topics).Error
	return topics, err
}

func (r *TopicRepo) Update(topic *model.Topic) error {
	return DB.Save(topic).Error
}

func (r *TopicRepo) Delete(id int64) error {
	return DB.Delete(&model.Topic{}, id).Error
}

func (r *TopicRepo) FindByName(name string) (*model.Topic, error) {
	var topic model.Topic
	err := DB.Where("LOWER(name) = LOWER(?)", name).First(&topic).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &topic, err
}

// FindOrCreateWithTx finds or creates a topic within a transaction.
// Uses the provided tx for all queries to maintain transaction isolation.
func (r *TopicRepo) FindOrCreateWithTx(tx *gorm.DB, name string) (*model.Topic, bool, error) {
	name = strings.ToLower(name)
	var topic model.Topic
	err := tx.Where("LOWER(name) = LOWER(?)", name).First(&topic).Error
	if err == nil {
		return &topic, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	topic = model.Topic{Name: name, Status: 1}
	if err := tx.Create(&topic).Error; err != nil {
		return nil, false, err
	}
	return &topic, true, nil
}

// ReplacePostTopics clears existing topic associations for a post and creates new ones within a transaction.
// It correctly decrements post_count for removed topics and increments for newly added ones.
func (r *TopicRepo) ReplacePostTopics(tx *gorm.DB, postID int64, topicIDs []int64) error {
	// Collect old topic IDs before deletion so we can decrement their counts
	var oldTopicIDs []int64
	if err := tx.Model(&model.TopicPost{}).Where("post_id = ?", postID).Pluck("topic_id", &oldTopicIDs).Error; err != nil {
		return err
	}

	// Build a set of new topic IDs for quick lookup
	newSet := make(map[int64]bool, len(topicIDs))
	for _, tid := range topicIDs {
		newSet[tid] = true
	}

	// Decrement post_count for topics that are being removed
	for _, tid := range oldTopicIDs {
		if !newSet[tid] {
			if err := tx.Model(&model.Topic{}).Where("id = ? AND post_count > 0", tid).
				UpdateColumn("post_count", gorm.Expr("post_count - 1")).Error; err != nil {
				slog.Warn("failed to decrement topic post count", "topic_id", tid, "err", err)
			}
		}
	}

	// Delete old topic-post associations
	if err := tx.Where("post_id = ?", postID).Delete(&model.TopicPost{}).Error; err != nil {
		return err
	}

	// Create new associations and increment post_count for topics that were not already present
	for _, tid := range topicIDs {
		if err := tx.Create(&model.TopicPost{TopicID: tid, PostID: postID}).Error; err != nil {
			return err
		}
		// Only increment if this topic wasn't already associated with the post
		if !contains(oldTopicIDs, tid) {
			if err := tx.Model(&model.Topic{}).Where("id = ?", tid).
				UpdateColumn("post_count", gorm.Expr("post_count + 1")).Error; err != nil {
				slog.Warn("failed to increment topic post count", "topic_id", tid, "err", err)
			}
		}
	}
	return nil
}

// contains checks if slice s contains element e.
func contains(s []int64, e int64) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}
