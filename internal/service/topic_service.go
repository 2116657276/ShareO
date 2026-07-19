package service

import (
	"github.com/zhoujianlin/ShareO/internal/model"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

type TopicService struct {
	topicRepo *repository.TopicRepo
	postRepo  *repository.PostRepo
}

func NewTopicService() *TopicService {
	return &TopicService{
		topicRepo: repository.NewTopicRepo(),
		postRepo:  repository.NewPostRepo(),
	}
}

type TopicPageData struct {
	Topic      *model.Topic
	Posts      []model.Post
	TotalPosts int64
}

func (s *TopicService) GetTopicPage(param string, page, pageSize int) (*TopicPageData, error) {
	// Try numeric ID first, then name lookup
	// Skip FindByID when param is not a valid numeric ID (parseInt64 returns 0)
	var topic *model.Topic
	id := parseInt64(param)
	if id > 0 {
		topic, _ = s.topicRepo.FindByID(id)
	}
	if topic == nil {
		var err error
		topic, err = s.topicRepo.FindByName(param)
		if err != nil {
			return nil, err
		}
	}
	if topic == nil {
		return nil, nil
	}

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}

	q := repository.FeedQuery{
		TopicID:  &topic.ID,
		Status:   model.StatusApproved,
		Sort:     model.SortLatest,
		Page:     page,
		PageSize: pageSize,
	}
	posts, total, err := s.postRepo.Feed(q)
	if err != nil {
		return nil, err
	}

	return &TopicPageData{
		Topic:      topic,
		Posts:      posts,
		TotalPosts: total,
	}, nil
}

func parseInt64(s string) int64 {
	var id int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		id = id*10 + int64(c-'0')
	}
	return id
}
