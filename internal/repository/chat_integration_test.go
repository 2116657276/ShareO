//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/zhoujianlin/ShareO/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func openChatIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("SHAREO_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set SHAREO_TEST_MYSQL_DSN to a migrated disposable *_test database")
	}
	parsed, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(parsed.DBName, "_test") {
		t.Fatalf("refusing integration test database %q: name must end in _test", parsed.DBName)
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestLightweightBaselineSchema(t *testing.T) {
	db := openChatIntegrationDB(t)
	expected := []string{
		"bot_replies", "comments", "conversation_members", "conversations",
		"favorites", "follows", "likes", "messages", "notifications", "post_images",
		"posts", "system_logs", "users",
	}
	var tables []string
	if err := db.Raw(`SELECT TABLE_NAME FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'`).Scan(&tables).Error; err != nil {
		t.Fatal(err)
	}
	sort.Strings(tables)
	if !reflect.DeepEqual(tables, expected) {
		t.Fatalf("business tables = %v, want %v", tables, expected)
	}

	removedColumns := map[string][]string{
		"posts":         {"favorite_count", "share_count", "is_repost", "repost_of_id", "repost_text"},
		"conversations": {"type", "title", "owner_id"},
	}
	for table, columns := range removedColumns {
		var count int64
		if err := db.Raw(`SELECT COUNT(*) FROM information_schema.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME IN ?`, table, columns).
			Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s still contains %d removed columns", table, count)
		}
	}
	var botCount int64
	if err := db.Model(&model.User{}).
		Where("username = ? AND is_bot = 1 AND status = ?", "shareo_bot", model.UserStatusActive).
		Count(&botCount).Error; err != nil || botCount != 1 {
		t.Fatalf("fixed bot count=%d err=%v", botCount, err)
	}
	users, err := NewChatRepo(db).SearchActiveUsers(context.Background(), "shareo_bot", 0, 10)
	if err != nil || len(users) != 1 || users[0].IsBot == 0 {
		t.Fatalf("bot search result=%v err=%v", users, err)
	}
}

func TestFavoriteRepoIsPrivateIdempotentAndFiltersInvisiblePosts(t *testing.T) {
	db := openChatIntegrationDB(t)
	previous := DB
	DB = db
	t.Cleanup(func() { DB = previous })

	ctx := context.Background()
	stamp := time.Now().UnixNano()
	owner := model.User{Username: fmt.Sprintf("fav_owner_%d", stamp), PasswordHash: "test", Status: model.UserStatusActive}
	viewer := model.User{Username: fmt.Sprintf("fav_viewer_%d", stamp), PasswordHash: "test", Status: model.UserStatusActive}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&viewer).Error; err != nil {
		t.Fatal(err)
	}
	posts := []model.Post{
		{UserID: owner.ID, Content: "可见收藏", Status: model.StatusApproved},
		{UserID: owner.ID, Content: "不可见收藏", Status: model.StatusRejected},
	}
	if err := db.Create(&posts).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Where("user_id IN ?", []int64{owner.ID, viewer.ID}).Delete(&model.Favorite{})
		db.Delete(&posts)
		db.Delete(&viewer)
		db.Delete(&owner)
	})

	repo := NewFavoriteRepo()
	for _, post := range posts {
		if err := repo.Ensure(ctx, viewer.ID, post.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.Ensure(ctx, viewer.ID, posts[0].ID); err != nil {
		t.Fatal(err)
	}
	list, total, err := repo.ListVisible(ctx, viewer.ID, 1, 20)
	if err != nil || total != 1 || len(list) != 1 || list[0].ID != posts[0].ID {
		t.Fatalf("visible favorites=%v total=%d err=%v", list, total, err)
	}
	otherList, otherTotal, err := repo.ListVisible(ctx, owner.ID, 1, 20)
	if err != nil || otherTotal != 0 || len(otherList) != 0 {
		t.Fatalf("other user's favorites leaked: list=%v total=%d err=%v", otherList, otherTotal, err)
	}
}

func TestBotReplyIsAtomicIdempotentAndFiltersCitations(t *testing.T) {
	db := openChatIntegrationDB(t)
	ctx := context.Background()
	repo := NewChatRepo(db)
	stamp := time.Now().UnixNano()
	user := model.User{
		Username: fmt.Sprintf("bot_it_%d", stamp), PasswordHash: "test", Status: model.UserStatusActive,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	var bot model.User
	if err := db.Where("username = ? AND is_bot = 1", "shareo_bot").First(&bot).Error; err != nil {
		t.Fatal(err)
	}
	posts := []model.Post{
		{UserID: user.ID, Content: "可引用正文", Status: model.StatusApproved},
		{UserID: user.ID, Content: "不可引用正文", Status: model.StatusRejected},
	}
	if err := db.Create(&posts).Error; err != nil {
		t.Fatal(err)
	}
	conversation, err := repo.EnsureDM(ctx, user.ID, bot.ID)
	if err != nil {
		t.Fatal(err)
	}
	source := &model.Message{ConversationID: conversation.ID, SenderID: user.ID, Content: "怎么拍？"}
	if err := repo.CreateMessage(ctx, source); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Where("source_message_id = ?", source.ID).Delete(&model.BotReply{})
		db.Where("conversation_id = ?", conversation.ID).Delete(&model.Message{})
		db.Where("conversation_id = ?", conversation.ID).Delete(&model.ConversationMember{})
		db.Delete(&model.Conversation{}, conversation.ID)
		db.Delete(&posts)
		db.Delete(&user)
	})

	reply, created, err := repo.CreateBotReply(ctx, source.ID, conversation.ID, "使用三脚架。", []model.BotCitation{
		{PostID: posts[0].ID, ChunkID: fmt.Sprintf("%d:0", posts[0].ID)},
		{PostID: posts[1].ID, ChunkID: fmt.Sprintf("%d:0", posts[1].ID)},
		{PostID: posts[0].ID, ChunkID: "999:0"},
	})
	if err != nil || !created {
		t.Fatalf("first bot reply=%v created=%v err=%v", reply, created, err)
	}
	var meta model.MessageMeta
	if reply.Meta == nil || json.Unmarshal([]byte(*reply.Meta), &meta) != nil {
		t.Fatalf("reply meta=%v", reply.Meta)
	}
	if len(meta.Citations) != 1 || meta.Citations[0].PostID != posts[0].ID {
		t.Fatalf("filtered citations=%v", meta.Citations)
	}

	duplicate, created, err := repo.CreateBotReply(ctx, source.ID, conversation.ID, "重复内容", nil)
	if err != nil || created || duplicate.ID != reply.ID || duplicate.Content != reply.Content {
		t.Fatalf("duplicate=%v created=%v err=%v", duplicate, created, err)
	}
	var replyCount int64
	db.Model(&model.BotReply{}).Where("source_message_id = ?", source.ID).Count(&replyCount)
	if replyCount != 1 {
		t.Fatalf("bot reply idempotency rows=%d", replyCount)
	}
}

func TestChatRepositoryIntegration(t *testing.T) {
	db := openChatIntegrationDB(t)
	ctx := context.Background()
	repo := NewChatRepo(db)
	stamp := time.Now().UnixNano()
	users := []model.User{
		{Username: fmt.Sprintf("chat_it_%d_a", stamp), PasswordHash: "test", Status: model.UserStatusActive},
		{Username: fmt.Sprintf("chat_it_%d_b", stamp), PasswordHash: "test", Status: model.UserStatusActive},
		{Username: fmt.Sprintf("chat_it_%d_c", stamp), PasswordHash: "test", Status: model.UserStatusActive},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	userIDs := []int64{users[0].ID, users[1].ID, users[2].ID}
	dmKeyValue := dmKey(users[0].ID, users[1].ID)
	t.Cleanup(func() {
		db.Where("sender_id IN ?", userIDs).Delete(&model.Message{})
		db.Where("user_id IN ?", userIDs).Delete(&model.ConversationMember{})
		db.Where("dm_key = ?", dmKeyValue).Delete(&model.Conversation{})
		db.Delete(&users)
	})

	// Concurrent creation must converge on one DM with both members.
	const workers = 8
	ids := make(chan int64, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conv, err := repo.EnsureDM(ctx, users[0].ID, users[1].ID)
			if err != nil {
				errs <- err
				return
			}
			ids <- conv.ID
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	var dmID int64
	for id := range ids {
		if dmID == 0 {
			dmID = id
		}
		if id != dmID {
			t.Fatalf("EnsureDM returned multiple IDs: %d and %d", dmID, id)
		}
	}
	var memberCount int64
	if err := db.Model(&model.ConversationMember{}).Where("conversation_id = ?", dmID).Count(&memberCount).Error; err != nil || memberCount != 2 {
		t.Fatalf("DM member count=%d err=%v", memberCount, err)
	}

	first := &model.Message{ConversationID: dmID, SenderID: users[0].ID, Content: "first"}
	second := &model.Message{ConversationID: dmID, SenderID: users[1].ID, Content: "second"}
	if err := repo.CreateMessage(ctx, first); err != nil {
		t.Fatal(err)
	}
	unread, err := repo.GetConversationUnreadCount(ctx, dmID, users[1].ID)
	if err != nil || unread != 1 {
		t.Fatalf("exact unread=%d want=1 err=%v", unread, err)
	}
	if err := repo.CreateMessage(ctx, second); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateReadMarker(ctx, dmID, users[0].ID, second.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateReadMarker(ctx, dmID, users[0].ID, first.ID); err != nil {
		t.Fatal(err)
	}
	var marker int64
	if err := db.Model(&model.ConversationMember{}).
		Where("conversation_id = ? AND user_id = ?", dmID, users[0].ID).
		Pluck("last_read_message_id", &marker).Error; err != nil || marker != second.ID {
		t.Fatalf("read marker regressed to %d, want %d, err=%v", marker, second.ID, err)
	}
	after, err := repo.GetMessages(ctx, dmID, 0, first.ID, 10)
	if err != nil || len(after) != 1 || after[0].ID != second.ID {
		t.Fatalf("after ordering=%v err=%v", after, err)
	}

	// A sender removed between authorization and insert must roll the message back.
	if err := db.Where("conversation_id = ? AND user_id = ?", dmID, users[0].ID).Delete(&model.ConversationMember{}).Error; err != nil {
		t.Fatal(err)
	}
	rolledBack := &model.Message{ConversationID: dmID, SenderID: users[0].ID, Content: "must rollback"}
	if err := repo.CreateMessage(ctx, rolledBack); err == nil {
		t.Fatal("expected message transaction to fail without sender membership")
	}
	var rolledBackCount int64
	db.Model(&model.Message{}).Where("conversation_id = ? AND content = ?", dmID, rolledBack.Content).Count(&rolledBackCount)
	if rolledBackCount != 0 {
		t.Fatal("failed message transaction left a message behind")
	}

	var fkCount int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
		WHERE CONSTRAINT_SCHEMA = DATABASE() AND CONSTRAINT_NAME IN
		('fk_conversation_members_conversation','fk_conversation_members_user','fk_messages_conversation','fk_messages_sender')`).
		Scan(&fkCount).Error; err != nil || fkCount != 4 {
		t.Fatalf("baseline chat constraints=%d err=%v", fkCount, err)
	}
}
