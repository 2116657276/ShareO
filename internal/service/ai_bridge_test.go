package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zhoujianlin/ShareO/internal/model"
)

type fakeBotOutbox struct {
	tasks     []model.BotTaskOutbox
	published []int64
	failed    []int64
}

func (o *fakeBotOutbox) CreateMessageWithBotTask(_ context.Context, _ *model.Message, _ bool) error {
	return nil
}

func (o *fakeBotOutbox) ListPendingBotTasks(context.Context, int) ([]model.BotTaskOutbox, error) {
	result := make([]model.BotTaskOutbox, 0, len(o.tasks))
	for _, task := range o.tasks {
		if task.Status == "pending" {
			result = append(result, task)
		}
	}
	return result, nil
}

func (o *fakeBotOutbox) MarkBotTaskPublished(_ context.Context, id int64) error {
	for i := range o.tasks {
		if o.tasks[i].ID == id {
			o.tasks[i].Status = "published"
		}
	}
	o.published = append(o.published, id)
	return nil
}

func (o *fakeBotOutbox) MarkBotTaskFailed(_ context.Context, id int64, _ string) error {
	for i := range o.tasks {
		if o.tasks[i].ID == id {
			o.tasks[i].Attempts++
		}
	}
	o.failed = append(o.failed, id)
	return nil
}

type fakeBotPublisher struct {
	calls chan [2]int64
	err   error
}

func (p *fakeBotPublisher) PublishBotTask(_ context.Context, conversationID, messageID int64) error {
	p.calls <- [2]int64{conversationID, messageID}
	return p.err
}

func TestOnlyMessagesToBotPublishTasksAndFailuresDoNotFailSend(t *testing.T) {
	bot := &model.User{ID: 2, Username: model.ShareOBotUsername, IsBot: 1, Status: model.UserStatusActive}
	user := &model.User{ID: 1, Username: "user", Status: model.UserStatusActive}
	repo := &fakeChatRepo{
		conversation: &model.Conversation{ID: 5},
		isMember:     true,
		members: []model.ConversationMember{
			{UserID: user.ID, User: user},
			{UserID: bot.ID, User: bot},
		},
	}
	publisher := &fakeBotPublisher{calls: make(chan [2]int64, 1), err: errors.New("redis down")}
	hub := &fakeHub{}
	svc := NewChatService(repo, fakePresence{}, hub, NewAIBridge(publisher))
	message, err := svc.SendMessage(context.Background(), user.ID, 5, "问题")
	if err != nil || message.ID != 99 {
		t.Fatalf("message=%v err=%v", message, err)
	}
	select {
	case call := <-publisher.calls:
		if call != [2]int64{5, 99} {
			t.Fatalf("published task=%v", call)
		}
	case <-time.After(time.Second):
		t.Fatal("bot task was not published")
	}

	ordinary := &model.User{ID: 3, Username: "other", Status: model.UserStatusActive}
	repo.members = []model.ConversationMember{{UserID: user.ID, User: user}, {UserID: ordinary.ID, User: ordinary}}
	if _, err := svc.SendMessage(context.Background(), user.ID, 5, "普通私聊"); err != nil {
		t.Fatal(err)
	}
	select {
	case call := <-publisher.calls:
		t.Fatalf("ordinary DM published bot task=%v", call)
	case <-time.After(50 * time.Millisecond):
	}

	otherBot := &model.User{ID: 4, Username: "other_bot", IsBot: 1, Status: model.UserStatusActive}
	repo.members = []model.ConversationMember{{UserID: otherBot.ID, User: otherBot}, {UserID: bot.ID, User: bot}}
	if _, err := svc.SendMessage(context.Background(), otherBot.ID, 5, "其他机器人消息"); err != nil {
		t.Fatal(err)
	}
	select {
	case call := <-publisher.calls:
		t.Fatalf("other bot message published bot task=%v", call)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestBotOutboxRetriesPendingTaskAndMarksItPublished(t *testing.T) {
	store := &fakeBotOutbox{tasks: []model.BotTaskOutbox{{ID: 7, ConversationID: 5, MessageID: 99, Status: "pending"}}}
	publisher := &fakeBotPublisher{calls: make(chan [2]int64, 2), err: errors.New("redis down")}
	bridge := NewAIBridge(publisher, store)

	bridge.publishPending(context.Background())
	if len(store.failed) != 1 || store.tasks[0].Attempts != 1 || len(store.published) != 0 {
		t.Fatalf("failed publish state: store=%+v", store)
	}
	publisher.err = nil
	bridge.publishPending(context.Background())
	if len(store.published) != 1 || store.tasks[0].Status != "published" {
		t.Fatalf("successful replay state: store=%+v", store)
	}
	if first := <-publisher.calls; first != [2]int64{5, 99} {
		t.Fatalf("first publish=%v", first)
	}
	if second := <-publisher.calls; second != [2]int64{5, 99} {
		t.Fatalf("second publish=%v", second)
	}
}

type fakeOutboxChatRepo struct {
	*fakeChatRepo
	enqueued bool
}

func (r *fakeOutboxChatRepo) CreateMessageWithBotTask(ctx context.Context, msg *model.Message, enqueue bool) error {
	r.enqueued = enqueue
	return r.fakeChatRepo.CreateMessage(ctx, msg)
}

func (r *fakeOutboxChatRepo) ListPendingBotTasks(context.Context, int) ([]model.BotTaskOutbox, error) {
	return nil, nil
}

func (r *fakeOutboxChatRepo) MarkBotTaskPublished(context.Context, int64) error      { return nil }
func (r *fakeOutboxChatRepo) MarkBotTaskFailed(context.Context, int64, string) error { return nil }

func TestChatMessageUsesOutboxInsteadOfDirectPublish(t *testing.T) {
	bot := &model.User{ID: 2, Username: model.ShareOBotUsername, IsBot: 1, Status: model.UserStatusActive}
	user := &model.User{ID: 1, Username: "user", Status: model.UserStatusActive}
	repo := &fakeOutboxChatRepo{fakeChatRepo: &fakeChatRepo{
		conversation: &model.Conversation{ID: 5},
		isMember:     true,
		members: []model.ConversationMember{
			{UserID: user.ID, User: user},
			{UserID: bot.ID, User: bot},
		},
	}}
	publisher := &fakeBotPublisher{calls: make(chan [2]int64, 1)}
	svc := NewChatService(repo, fakePresence{}, &fakeHub{}, NewAIBridge(publisher, repo))
	if _, err := svc.SendMessage(context.Background(), user.ID, 5, "问题"); err != nil {
		t.Fatal(err)
	}
	if !repo.enqueued {
		t.Fatal("bot task was not included in the message transaction")
	}
	select {
	case call := <-publisher.calls:
		t.Fatalf("outbox path published directly: %v", call)
	case <-time.After(50 * time.Millisecond):
	}
}
