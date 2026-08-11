//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/zhoujianlin/ShareO/internal/model"
)

func TestCommentParentBoundaryAndCascadeKeepCountConsistent(t *testing.T) {
	db := openChatIntegrationDB(t)
	previous := DB
	DB = db
	t.Cleanup(func() { DB = previous })

	stamp := time.Now().UnixNano()
	user := model.User{Username: fmt.Sprintf("comment_it_%d", stamp), PasswordHash: "test", Status: model.UserStatusActive}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	posts := []model.Post{
		{UserID: user.ID, Content: "post one", Status: model.StatusApproved},
		{UserID: user.ID, Content: "post two", Status: model.StatusApproved},
	}
	if err := db.Create(&posts).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Where("post_id IN ?", []int64{posts[0].ID, posts[1].ID}).Delete(&model.Comment{})
		db.Delete(&posts)
		db.Delete(&user)
	})

	repo := NewCommentRepo()
	parent := &model.Comment{PostID: posts[0].ID, UserID: user.ID, Content: "parent"}
	if err := repo.Create(parent); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Post{}).Where("id = ?", posts[0].ID).
		Update("status", model.StatusPending).Error; err != nil {
		t.Fatal(err)
	}
	comments, total, err := repo.FindByPostIDForUser(posts[0].ID, 1, 20, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 0 || total != 0 {
		t.Fatalf("comments for a pending post remained visible: len=%d total=%d", len(comments), total)
	}
	if err := db.Model(&model.Post{}).Where("id = ?", posts[0].ID).
		Update("status", model.StatusApproved).Error; err != nil {
		t.Fatal(err)
	}
	otherPostComment := &model.Comment{PostID: posts[1].ID, UserID: user.ID, Content: "other"}
	if err := repo.Create(otherPostComment); err != nil {
		t.Fatal(err)
	}
	if found, err := repo.FindVisibleTopLevelForPost(context.Background(), posts[0].ID, otherPostComment.ID); err != nil {
		t.Fatal(err)
	} else if found != nil {
		t.Fatalf("cross-post parent accepted: %+v", found)
	}

	child := &model.Comment{PostID: posts[0].ID, UserID: user.ID, ParentID: &parent.ID, Content: "child"}
	if err := repo.Create(child); err != nil {
		t.Fatal(err)
	}
	if err := repo.SoftDelete(parent.ID, user.ID); err != nil {
		t.Fatal(err)
	}
	var storedChild model.Comment
	if err := db.First(&storedChild, child.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedChild.IsDeleted != 1 {
		t.Fatalf("child was not cascaded: %+v", storedChild)
	}
	var post model.Post
	if err := db.First(&post, posts[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	if post.CommentCount != 0 {
		t.Fatalf("post comment count=%d, want 0", post.CommentCount)
	}
}
