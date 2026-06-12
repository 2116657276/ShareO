package repository

import (
	"github.com/zhoujianlin/ShareO/internal/model"
)

type TopicRepo struct{}

func NewTopicRepo() *TopicRepo { return &TopicRepo{} }

func (r *TopicRepo) Create(topic *model.Topic) error {
	return DB.Create(topic).Error
}

func (r *TopicRepo) FindByID(id int64) (*model.Topic, error) {
	var topic model.Topic
	err := DB.First(&topic, id).Error
	if err != nil {
		return nil, err
	}
	return &topic, nil
}

func (r *TopicRepo) List(onlyActive bool) ([]model.Topic, error) {
	var topics []model.Topic
	query := DB.Model(&model.Topic{})
	if onlyActive {
		query = query.Where("status = 1")
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

func (r *TopicRepo) AddPostToTopic(topicID, postID int64) error {
	tp := model.TopicPost{TopicID: topicID, PostID: postID}
	if err := DB.Create(&tp).Error; err != nil {
		return err
	}
	r.IncrementPostCount(topicID)
	return nil
}
