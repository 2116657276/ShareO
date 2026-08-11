package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type BotTaskPublisher interface {
	PublishBotTask(context.Context, int64, int64) error
}

// AIBridge keeps Redis/AI latency and failure outside the chat write path. In
// production the outbox is supplied by ChatRepo; direct publication remains a
// compatibility fallback for lightweight repositories and tests.
type AIBridge struct {
	publisher BotTaskPublisher
	outbox    BotTaskOutboxRepository
	startOnce sync.Once
}

func NewAIBridge(publisher BotTaskPublisher, outboxes ...BotTaskOutboxRepository) *AIBridge {
	bridge := &AIBridge{publisher: publisher}
	if len(outboxes) > 0 {
		bridge.outbox = outboxes[0]
	}
	return bridge
}

func (b *AIBridge) PublishBotTask(conversationID, messageID int64) {
	if b == nil || b.publisher == nil || b.outbox != nil {
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

// Start replays pending tasks immediately and then polls at a low frequency.
// The application intentionally runs one bridge per Go process, matching the
// single-worker runtime boundary documented for this project.
func (b *AIBridge) Start(ctx context.Context) {
	if b == nil || b.publisher == nil || b.outbox == nil {
		return
	}
	b.startOnce.Do(func() {
		go func() {
			b.publishPending(ctx)
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					b.publishPending(ctx)
				}
			}
		}()
	})
}

func (b *AIBridge) publishPending(ctx context.Context) {
	tasks, err := b.outbox.ListPendingBotTasks(ctx, 20)
	if err != nil {
		slog.Warn("bot outbox scan failed", "err", err)
		return
	}
	for _, task := range tasks {
		publishCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := b.publisher.PublishBotTask(publishCtx, task.ConversationID, task.MessageID)
		cancel()
		if err != nil {
			slog.Warn("bot outbox publish failed", "outbox_id", task.ID, "conversation_id", task.ConversationID, "message_id", task.MessageID, "err", err)
			if markErr := b.outbox.MarkBotTaskFailed(ctx, task.ID, err.Error()); markErr != nil {
				slog.Warn("bot outbox failure state update failed", "outbox_id", task.ID, "err", markErr)
			}
			continue
		}
		if err := b.outbox.MarkBotTaskPublished(ctx, task.ID); err != nil {
			slog.Warn("bot outbox publish state update failed", "outbox_id", task.ID, "err", err)
		}
	}
}
