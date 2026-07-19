package queue

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const (
	StreamIndexPost = "shareo:stream:index_post"
	StreamBotTasks  = "shareo:stream:bot_tasks"
	ConsumerGroup   = "ai-workers"
)

type Queue struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Queue {
	return &Queue{rdb: rdb}
}

// PublishIndexPost publishes an index action to the post indexing stream.
func (q *Queue) PublishIndexPost(ctx context.Context, action string, postID int64) error {
	return q.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamIndexPost,
		Values: map[string]interface{}{"action": action, "post_id": fmt.Sprintf("%d", postID)},
	}).Err()
}

// PublishBotTask publishes a bot task to the bot tasks stream.
func (q *Queue) PublishBotTask(ctx context.Context, convID, msgID int64) error {
	return q.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamBotTasks,
		Values: map[string]interface{}{
			"conversation_id": fmt.Sprintf("%d", convID),
			"message_id":      fmt.Sprintf("%d", msgID),
		},
	}).Err()
}
