package repository

import (
	"errors"
	"strings"

	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/gorm"
)

type TopicRepo struct{}

func NewTopicRepo() *TopicRepo { return &TopicRepo{} }

func (r *TopicRepo) CountByStatus(status int8) int64 {
	var count int64
	DB.Model(&model.Topic{}).Where("status = ?", status).Count(&count)
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

func (r *TopicRepo) IncrementPostCount(topicID int64) {
	DB.Model(&model.Topic{}).Where("id = ?", topicID).UpdateColumn("post_count", DB.Raw("post_count + 1"))
}

func (r *TopicRepo) DecrementPostCount(topicID int64) {
	DB.Model(&model.Topic{}).Where("id = ? AND post_count > 0", topicID).UpdateColumn("post_count", DB.Raw("post_count - 1"))
}

func (r *TopicRepo) FindByName(name string) (*model.Topic, error) {
	var topic model.Topic
	err := DB.Where("LOWER(name) = LOWER(?)", name).First(&topic).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &topic, err
}

func (r *TopicRepo) FindOrCreate(name string) (*model.Topic, bool, error) {
	name = strings.ToLower(name)
	topic, err := r.FindByName(name)
	if err != nil {
		return nil, false, err
	}
	if topic != nil {
		return topic, false, nil
	}
	topic = &model.Topic{Name: name, Status: 1}
	if err := r.Create(topic); err != nil {
		return nil, false, err
	}
	return topic, true, nil
}

func (r *TopicRepo) AddPostToTopic(topicID, postID int64) error {
	tp := model.TopicPost{TopicID: topicID, PostID: postID}
	if err := DB.Create(&tp).Error; err != nil {
		return err
	}
	r.IncrementPostCount(topicID)
	return nil
}

// FindOrCreateWithTx finds or creates a topic within a transaction.
func (r *TopicRepo) FindOrCreateWithTx(tx *gorm.DB, name string) (*model.Topic, bool, error) {
	name = strings.ToLower(name)
	topic, err := r.FindByName(name)
	if err != nil {
		return nil, false, err
	}
	if topic != nil {
		return topic, false, nil
	}
	topic = &model.Topic{Name: name, Status: 1}
	if err := tx.Create(topic).Error; err != nil {
		return nil, false, err
	}
	return topic, true, nil
}

// ReplacePostTopics clears existing topic associations for a post and creates new ones within a transaction.
func (r *TopicRepo) ReplacePostTopics(tx *gorm.DB, postID int64, topicIDs []int64) error {
	if err := tx.Where("post_id = ?", postID).Delete(&model.TopicPost{}).Error; err != nil {
		return err
	}
	for _, tid := range topicIDs {
		if err := tx.Create(&model.TopicPost{TopicID: tid, PostID: postID}).Error; err != nil {
			return err
		}
		tx.Model(&model.Topic{}).Where("id = ?", tid).UpdateColumn("post_count", gorm.Expr("post_count + 1"))
	}
	return nil
}
