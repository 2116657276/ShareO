package service

import (
	"context"
	"log/slog"

	"github.com/zhoujianlin/ShareO/internal/pkg/queue"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

// publishIndexAction keeps the database request independent from the optional
// AI worker. A Redis outage must not turn a successful post moderation/delete
// operation into a client-visible failure; the backfill command can repair the
// stream later.
func publishIndexAction(action string, postID int64) {
	if repository.RDB == nil {
		return
	}
	if err := queue.New(repository.RDB).PublishIndexPost(context.Background(), action, postID); err != nil {
		slog.Warn("failed to publish image index action", "action", action, "post_id", postID, "err", err)
	}
}
