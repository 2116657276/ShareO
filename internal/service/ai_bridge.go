package service

import (
	"context"
	"log/slog"
	"time"
)

type BotTaskPublisher interface {
	PublishBotTask(context.Context, int64, int64) error
}

// AIBridge keeps Redis/AI latency and failure outside the chat write path.
type AIBridge struct {
	publisher BotTaskPublisher
}

func NewAIBridge(publisher BotTaskPublisher) *AIBridge {
	return &AIBridge{publisher: publisher}
}

func (b *AIBridge) PublishBotTask(conversationID, messageID int64) {
	if b == nil || b.publisher == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := b.publisher.PublishBotTask(ctx, conversationID, messageID); err != nil {
			slog.Warn("bot task publish failed after message commit",
				"conversation_id", conversationID, "message_id", messageID, "err", err)
		}
	}()
}
