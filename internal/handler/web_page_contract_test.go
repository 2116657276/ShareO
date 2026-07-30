package handler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readWebTemplate(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("../../web/templates", name))
	if err != nil {
		t.Fatalf("read template %s: %v", name, err)
	}
	return string(content)
}

func TestWebTemplatesUseSharedShell(t *testing.T) {
	root := filepath.Join("../../web/templates")
	var checked int
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".html" || filepath.Base(path) == "header.html" || filepath.Base(path) == "footer.html" {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		page := string(content)
		if !strings.Contains(page, `{{ template "header" . }}`) || !strings.Contains(page, `{{ template "footer" . }}`) {
			t.Errorf("%s must use the shared header and footer", path)
		}
		for _, forbidden := range []string{"Authorization", "X-Internal-Token", "SHAREO_AI_INTERNAL_TOKEN", "DEEPSEEK_API_KEY", "api_key"} {
			if strings.Contains(strings.ToLower(page), strings.ToLower(forbidden)) {
				t.Errorf("%s exposes forbidden frontend term %q", path, forbidden)
			}
		}
		if strings.Contains(page, "style=") {
			t.Errorf("%s contains inline style; use shared design-system classes", path)
		}
		checked++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 16 {
		t.Fatalf("checked %d templates, want at least 16", checked)
	}
}

func TestSharedShellContract(t *testing.T) {
	header := readWebTemplate(t, "layout/header.html")
	for _, required := range []string{"skip-link", "main-content", "app-sidebar", "mobile-bottom-nav", "/home", "/following", "/search/images", "/chat", "/user/{{ .user_id }}", "aria-label=\"主导航\""} {
		if !strings.Contains(header, required) {
			t.Fatalf("shared header missing %q", required)
		}
	}
	previous := -1
	for _, href := range []string{`href="/home"`, `href="/following"`, `href="/search/images"`, `href="/chat"`, `href="/user/{{ .user_id }}"`} {
		position := strings.Index(header, href)
		if position < 0 || position <= previous {
			t.Fatalf("primary navigation is missing or out of order at %q", href)
		}
		previous = position
	}
	if strings.Contains(header, "app-sidebar-tools") || strings.Contains(header, "/notifications") {
		t.Fatal("global publish/notification/chat tools must not remain in the shared navigation")
	}
	if strings.Contains(header, "app-account") || strings.Contains(header, "app-account-link") {
		t.Fatal("desktop sidebar must not duplicate the profile navigation entry")
	}
	for _, asset := range []string{
		"/static/vendor/bootstrap/css/bootstrap.min.css",
		"/static/vendor/bootstrap-icons/font/bootstrap-icons.css",
		"/static/vendor/alpinejs/alpine.min.js",
	} {
		if !strings.Contains(header, asset) {
			t.Fatalf("shared header missing local frontend dependency %q", asset)
		}
	}
	if strings.Contains(header, "https://") || strings.Contains(header, "http://") {
		t.Fatal("shared header must not depend on external runtime assets")
	}

	footer := readWebTemplate(t, "layout/footer.html")
	if !strings.Contains(footer, "site-footer") {
		t.Fatal("shared footer missing site-footer hook")
	}
	if !strings.Contains(footer, "/static/vendor/bootstrap/js/bootstrap.bundle.min.js") {
		t.Fatal("shared footer missing local Bootstrap bundle")
	}
	if strings.Contains(footer, "https://") || strings.Contains(footer, "http://") {
		t.Fatal("shared footer must not depend on external runtime assets")
	}
}

func TestPageStateContracts(t *testing.T) {
	cases := map[string][]string{
		"feed/feed.html":           {"state-panel", "loading", "重新加载", "shareo.feed.layout", "setLayout('two')", "mediumURL", "rotate"},
		"search/image_search.html": {"imageSearchEmpty", "imageSearchUnavailable", "重新尝试", "/api/v1/search", "/api/v1/search/images", "data-mode=\"posts\"", "data-mode=\"images\"", "问问小O", "/chat?bot=agent"},
		"user/following.html":      {"loading-block", "empty-block", "errorMessage", "/api/v1/users/${this.currentUserID}/following", "/api/v1/feed/following", "following-people"},
		"user/notifications.html":  {"loading-block", "empty-block", "errorMessage"},
		"chat/chat.html":           {"chat-page-shell", "errorMessage", "conversation-list-scroll", "visibleConvs", "chat-back-button", "requestedBotAgent", "isBotConversationData", "问问小O", "ai_mode"},
		"layout/404.html":          {"state-panel", "页面不存在"},
		"admin/admin_review.html":  {"CONTENT REVIEW", "没有待审核帖子"},
		"post/create_post.html":    {"upload-zone", "上传中"},
	}
	for name, required := range cases {
		page := readWebTemplate(t, name)
		for _, marker := range required {
			if !strings.Contains(page, marker) {
				t.Errorf("%s missing state marker %q", name, marker)
			}
		}
	}
}

func TestRemovedPageControlsStayRemoved(t *testing.T) {
	following := readWebTemplate(t, "user/following.html")
	if strings.Contains(following, "全部关注") || strings.Contains(following, "/api/v1/conversations") {
		t.Fatal("following page must not expose the removed expanded list or direct-message action")
	}

	chat := readWebTemplate(t, "chat/chat.html")
	for _, removed := range []string{`aria-label="打开 AI 助手"`, `aria-label="发起私聊"`, `placeholder="搜索用户`} {
		if strings.Contains(chat, removed) {
			t.Fatalf("chat list still exposes removed control %q", removed)
		}
	}
}

func TestFooterAndMyPageContracts(t *testing.T) {
	footer := readWebTemplate(t, "layout/footer.html")
	if !strings.Contains(footer, "site-footer") || !strings.Contains(footer, "public-footer") {
		t.Fatal("footer variants are missing")
	}
	for _, marker := range []string{"data-chat-unread", "data-comment-unread", "/api/v1/conversations/unread-count", "/api/v1/notifications/unread-count"} {
		if !strings.Contains(footer, marker) && !strings.Contains(readWebTemplate(t, "layout/header.html"), marker) {
			t.Fatalf("shared shell missing unread marker %q", marker)
		}
	}
	profile := readWebTemplate(t, "user/user_profile.html")
	for _, marker := range []string{"通知", "作品", "点赞", "收藏", "/api/v1/favorites", "profile-tabs", "profile-content-toolbar", "layout-toggle"} {
		if !strings.Contains(profile, marker) {
			t.Fatalf("my page missing %q", marker)
		}
	}
}

func TestThirdRoundUIContracts(t *testing.T) {
	chat := readWebTemplate(t, "chat/chat.html")
	if !strings.Contains(chat, "c.last_message && !this.isBotConversationData(c)") {
		t.Fatal("normal conversation list must hide shareo_bot")
	}
	if strings.Contains(chat, `class="chat-detail-avatar" :src="conversationAvatar(activeConversation)"`) &&
		!strings.Contains(chat, `x-show="!isBotConversation()"`) {
		t.Fatal("bot conversation header must hide the generic peer avatar")
	}

	css, err := os.ReadFile("../../web/static/css/style.css")
	if err != nil {
		t.Fatal(err)
	}
	stylesheet := string(css)
	for _, marker := range []string{
		"transform: scale(1.09)",
		".profile-content-toolbar",
		".chat-bot-heading",
	} {
		if !strings.Contains(stylesheet, marker) {
			t.Fatalf("stylesheet missing third-round UI marker %q", marker)
		}
	}
}

func TestPostDetailInteractionContracts(t *testing.T) {
	page := readWebTemplate(t, "post/post_detail.html")
	for _, marker := range []string{"/favorite", "IsFavorited", `/user/`} {
		if !strings.Contains(page, marker) {
			t.Fatalf("post detail missing interaction marker %q", marker)
		}
	}
}
