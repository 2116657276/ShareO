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
		checked++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 15 {
		t.Fatalf("checked %d templates, want at least 15", checked)
	}
}

func TestSharedShellContract(t *testing.T) {
	header := readWebTemplate(t, "layout/header.html")
	for _, required := range []string{"skip-link", "main-content", "/search/images", "aria-label=\"主导航\""} {
		if !strings.Contains(header, required) {
			t.Fatalf("shared header missing %q", required)
		}
	}

	footer := readWebTemplate(t, "layout/footer.html")
	if !strings.Contains(footer, "site-footer") {
		t.Fatal("shared footer missing site-footer hook")
	}
}

func TestPageStateContracts(t *testing.T) {
	cases := map[string][]string{
		"feed/feed.html":           {"state-panel", "loading", "重新加载"},
		"search/image_search.html": {"imageSearchEmpty", "imageSearchUnavailable", "重新尝试"},
		"user/notifications.html":  {"loading-block", "empty-block", "errorMessage"},
		"chat/chat.html":           {"chat-page-shell", "errorMessage", "深度分析"},
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
