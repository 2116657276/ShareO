package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zhoujianlin/ShareO/internal/model"
)

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
