package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/zhoujianlin/ShareO/internal/model"
)

func TestToPublicUserOmitsPrivateAccountFields(t *testing.T) {
	user := &model.User{
		ID: 7, Username: "portfolio", PasswordHash: "secret-hash",
		Email: "portfolio@example.com", Role: model.RoleAdmin, Status: model.UserStatusBanned,
		IsBot: 1, Posts: []model.Post{{ID: 99}},
	}
	encoded, err := json.Marshal(ToPublicUser(user))
	if err != nil {
		t.Fatalf("marshal public user: %v", err)
	}
	body := string(encoded)
	for _, forbidden := range []string{"password_hash", "secret-hash", "email", "role", "status", "posts"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("public user contains private field %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, `"is_bot":true`) {
		t.Fatalf("public user lost intended bot marker: %s", body)
	}
}

func TestToPublicPostOmitsModerationAndNestedPersistenceFields(t *testing.T) {
	post := &model.Post{
		ID: 9, UserID: 7, Content: "visible", Status: model.StatusApproved,
		ReviewComment: "internal review", IsDeleted: 1,
		User:            &model.User{ID: 7, Username: "portfolio", Email: "private@example.com", Role: model.RoleAdmin},
		FeaturedComment: &model.Comment{ID: 3, Content: "hello", User: &model.User{ID: 8, Username: "commenter", Email: "private@example.com"}},
	}
	encoded, err := json.Marshal(ToPublicPost(post))
	if err != nil {
		t.Fatalf("marshal public post: %v", err)
	}
	body := string(encoded)
	for _, forbidden := range []string{"review_comment", "is_deleted", "private@example.com", "role", "password_hash"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("public post contains private field %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, `"content":"visible"`) {
		t.Fatalf("public post lost content: %s", body)
	}
}

func TestToPublicChatModelsOmitPrivateUserFields(t *testing.T) {
	privateUser := &model.User{
		ID: 8, Username: "friend", Email: "private@example.com",
		PasswordHash: "secret-hash", Role: model.RoleAdmin, Status: model.UserStatusBanned,
	}
	message := &model.Message{
		ID: 12, ConversationID: 4, SenderID: privateUser.ID,
		Content: "hello", Sender: privateUser,
	}
	conversation := &model.Conversation{
		ID: 4, DmKey: "2:8", Members: []model.ConversationMember{{
			ID: 1, ConversationID: 4, UserID: privateUser.ID,
			User: privateUser,
		}}, LastMessage: message,
	}

	encoded, err := json.Marshal(struct {
		Conversation *PublicConversation `json:"conversation"`
		Message      *PublicMessage      `json:"message"`
	}{
		Conversation: ToPublicConversationModel(conversation),
		Message:      ToPublicMessage(message),
	})
	if err != nil {
		t.Fatalf("marshal public chat models: %v", err)
	}
	body := string(encoded)
	for _, forbidden := range []string{"private@example.com", "secret-hash", `"role"`, `"status"`, `"dm_key"`} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("chat response contains private field %q: %s", forbidden, body)
		}
	}
	for _, expected := range []string{`"username":"friend"`, `"content":"hello"`, `"sender_id":8`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("chat response lost expected field %q: %s", expected, body)
		}
	}
}
