package service

import (
	"time"

	"github.com/zhoujianlin/ShareO/internal/model"
)

// PublicUser is the deliberately small user shape exposed by community APIs.
// Account contact data, moderation state and role stay behind authenticated
// profile or administrator responses.
type PublicUser struct {
	ID         int64      `json:"id"`
	Username   string     `json:"username"`
	AvatarURL  string     `json:"avatar_url"`
	Bio        string     `json:"bio"`
	IsBot      bool       `json:"is_bot,omitempty"`
	FollowedAt *time.Time `json:"followed_at,omitempty"`
}

// AuthUser is the private profile shape returned after authentication. It is
// separate from PublicUser so adding a field here cannot widen community API
// responses by accident.
type AuthUser struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	AvatarURL string    `json:"avatar_url"`
	Bio       string    `json:"bio"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// PublicComment is the recursive, presentation-safe comment shape used by
// feed previews and public comment APIs.
type PublicComment struct {
	ID         int64            `json:"id"`
	PostID     int64            `json:"post_id"`
	UserID     int64            `json:"user_id"`
	ParentID   *int64           `json:"parent_id"`
	ReplyToUID *int64           `json:"reply_to_uid"`
	Content    string           `json:"content"`
	LikeCount  int              `json:"like_count"`
	CreatedAt  time.Time        `json:"created_at"`
	User       *PublicUser      `json:"user,omitempty"`
	ReplyTo    *PublicUser      `json:"reply_to,omitempty"`
	Children   []*PublicComment `json:"children,omitempty"`
	IsLiked    bool             `json:"is_liked"`
}

// PublicPost is the community read model. Moderation notes, deletion flags,
// reviewer associations and persistence timestamps not needed by readers are
// intentionally excluded.
type PublicPost struct {
	ID              int64             `json:"id"`
	UserID          int64             `json:"user_id"`
	Content         string            `json:"content"`
	CoverImage      string            `json:"cover_image"`
	ViewCount       int               `json:"view_count"`
	LikeCount       int               `json:"like_count"`
	CommentCount    int               `json:"comment_count"`
	CreatedAt       time.Time         `json:"created_at"`
	User            *PublicUser       `json:"user,omitempty"`
	Images          []model.PostImage `json:"images,omitempty"`
	FeaturedComment *PublicComment    `json:"featured_comment,omitempty"`
	IsLiked         bool              `json:"is_liked"`
	IsFavorited     bool              `json:"is_favorited"`
}

type PublicNotification struct {
	ID        int64       `json:"id"`
	UserID    int64       `json:"user_id"`
	Type      string      `json:"type"`
	ActorID   int64       `json:"actor_id"`
	TargetID  int64       `json:"target_id"`
	IsRead    int8        `json:"is_read"`
	CreatedAt time.Time   `json:"created_at"`
	Actor     *PublicUser `json:"actor,omitempty"`
}

// PublicMessage is the authenticated chat read model. Message metadata is
// retained for citation and Agent trace rendering, while the associated user
// is narrowed to the same safe shape used by community APIs.
type PublicMessage struct {
	ID             int64       `json:"id"`
	ConversationID int64       `json:"conversation_id"`
	SenderID       int64       `json:"sender_id"`
	Content        string      `json:"content"`
	Meta           *string     `json:"meta,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	Sender         *PublicUser `json:"sender,omitempty"`
}

type PublicConversationMember struct {
	ID                int64       `json:"id"`
	ConversationID    int64       `json:"conversation_id"`
	UserID            int64       `json:"user_id"`
	LastReadMessageID int64       `json:"last_read_message_id"`
	JoinedAt          time.Time   `json:"joined_at"`
	User              *PublicUser `json:"user,omitempty"`
}

// PublicConversation keeps the fields required by the chat UI while avoiding
// the deterministic DM key and persistence associations from the model layer.
type PublicConversation struct {
	ID          int64                      `json:"id"`
	CreatedAt   time.Time                  `json:"created_at"`
	UpdatedAt   time.Time                  `json:"updated_at"`
	Title       string                     `json:"title"`
	LastMessage *PublicMessage             `json:"last_message,omitempty"`
	Members     []PublicConversationMember `json:"members,omitempty"`
	UnreadCount int64                      `json:"unread_count"`
	Online      bool                       `json:"online"`
	FollowedAt  *time.Time                 `json:"followed_at,omitempty"`
}

func ToPublicUser(user *model.User) *PublicUser {
	if user == nil {
		return nil
	}
	return &PublicUser{
		ID: user.ID, Username: user.Username, AvatarURL: user.AvatarURL,
		Bio: user.Bio, IsBot: user.IsBot != 0, FollowedAt: user.FollowedAt,
	}
}

func ToAuthUser(user *model.User) *AuthUser {
	if user == nil {
		return nil
	}
	return &AuthUser{
		ID: user.ID, Username: user.Username, Email: user.Email,
		AvatarURL: user.AvatarURL, Bio: user.Bio, Role: user.Role,
		CreatedAt: user.CreatedAt,
	}
}

func ToPublicComment(comment *model.Comment) *PublicComment {
	if comment == nil {
		return nil
	}
	result := &PublicComment{
		ID: comment.ID, PostID: comment.PostID, UserID: comment.UserID,
		ParentID: comment.ParentID, ReplyToUID: comment.ReplyToUID,
		Content: comment.Content, LikeCount: comment.LikeCount,
		CreatedAt: comment.CreatedAt, User: ToPublicUser(comment.User),
		ReplyTo: ToPublicUser(comment.ReplyTo), IsLiked: comment.IsLiked,
	}
	if len(comment.Children) > 0 {
		result.Children = make([]*PublicComment, 0, len(comment.Children))
		for _, child := range comment.Children {
			result.Children = append(result.Children, ToPublicComment(child))
		}
	}
	return result
}

func ToPublicComments(comments []model.Comment) []*PublicComment {
	result := make([]*PublicComment, 0, len(comments))
	for i := range comments {
		result = append(result, ToPublicComment(&comments[i]))
	}
	return result
}

func ToPublicPost(post *model.Post) *PublicPost {
	if post == nil {
		return nil
	}
	result := &PublicPost{
		ID: post.ID, UserID: post.UserID, Content: post.Content,
		CoverImage: post.CoverImage, ViewCount: post.ViewCount,
		LikeCount: post.LikeCount, CommentCount: post.CommentCount,
		CreatedAt: post.CreatedAt, User: ToPublicUser(post.User),
		Images: post.Images, IsLiked: post.IsLiked, IsFavorited: post.IsFavorited,
		FeaturedComment: ToPublicComment(post.FeaturedComment),
	}
	return result
}

func ToPublicPosts(posts []model.Post) []*PublicPost {
	result := make([]*PublicPost, 0, len(posts))
	for i := range posts {
		result = append(result, ToPublicPost(&posts[i]))
	}
	return result
}

func ToPublicUsers(users []model.User) []*PublicUser {
	result := make([]*PublicUser, 0, len(users))
	for i := range users {
		result = append(result, ToPublicUser(&users[i]))
	}
	return result
}

func ToPublicNotifications(notifications []model.Notification) []*PublicNotification {
	result := make([]*PublicNotification, 0, len(notifications))
	for i := range notifications {
		notification := &notifications[i]
		result = append(result, &PublicNotification{
			ID: notification.ID, UserID: notification.UserID, Type: notification.Type,
			ActorID: notification.ActorID, TargetID: notification.TargetID,
			IsRead: notification.IsRead, CreatedAt: notification.CreatedAt,
			Actor: ToPublicUser(notification.Actor),
		})
	}
	return result
}

func ToPublicMessage(message *model.Message) *PublicMessage {
	if message == nil {
		return nil
	}
	return &PublicMessage{
		ID: message.ID, ConversationID: message.ConversationID,
		SenderID: message.SenderID, Content: message.Content,
		Meta: message.Meta, CreatedAt: message.CreatedAt,
		Sender: ToPublicUser(message.Sender),
	}
}

func ToPublicMessages(messages []model.Message) []*PublicMessage {
	result := make([]*PublicMessage, 0, len(messages))
	for i := range messages {
		result = append(result, ToPublicMessage(&messages[i]))
	}
	return result
}

func ToPublicConversationMember(member model.ConversationMember) PublicConversationMember {
	return PublicConversationMember{
		ID: member.ID, ConversationID: member.ConversationID,
		UserID: member.UserID, LastReadMessageID: member.LastReadMessageID,
		JoinedAt: member.JoinedAt, User: ToPublicUser(member.User),
	}
}

func ToPublicConversationModel(conversation *model.Conversation) *PublicConversation {
	if conversation == nil {
		return nil
	}
	result := &PublicConversation{
		ID: conversation.ID, CreatedAt: conversation.CreatedAt,
		UpdatedAt: conversation.UpdatedAt, Title: conversation.Title,
		LastMessage: ToPublicMessage(conversation.LastMessage),
		UnreadCount: conversation.UnreadCount,
	}
	if len(conversation.Members) > 0 {
		result.Members = make([]PublicConversationMember, 0, len(conversation.Members))
		for _, member := range conversation.Members {
			result.Members = append(result.Members, ToPublicConversationMember(member))
		}
	}
	return result
}

func ToPublicConversationWithMeta(conversation ConversationWithMeta) *PublicConversation {
	result := &PublicConversation{
		ID: conversation.ID, CreatedAt: conversation.CreatedAt,
		UpdatedAt: conversation.UpdatedAt, Title: conversation.Title,
		LastMessage: ToPublicMessage(conversation.LastMessage),
		UnreadCount: conversation.UnreadCount, Online: conversation.Online,
		FollowedAt: conversation.FollowedAt,
	}
	if len(conversation.Members) > 0 {
		result.Members = make([]PublicConversationMember, 0, len(conversation.Members))
		for _, member := range conversation.Members {
			result.Members = append(result.Members, ToPublicConversationMember(member))
		}
	}
	return result
}

func ToPublicConversations(conversations []ConversationWithMeta) []*PublicConversation {
	result := make([]*PublicConversation, 0, len(conversations))
	for _, conversation := range conversations {
		result = append(result, ToPublicConversationWithMeta(conversation))
	}
	return result
}
