package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func TestCalcPages(t *testing.T) {
	tests := []struct {
		total    int64
		pageSize int
		want     int
	}{
		{0, 20, 0},
		{1, 20, 1},
		{20, 20, 1},
		{21, 20, 2},
		{100, 20, 5},
		{50, 50, 1},
		{51, 50, 2},
		{0, 10, 0},
	}

	for _, tt := range tests {
		got := calcPages(tt.total, tt.pageSize)
		if got != tt.want {
			t.Errorf("calcPages(%d, %d) = %d, want %d", tt.total, tt.pageSize, got, tt.want)
		}
	}
}

func TestGetPage(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test?page=5", nil)

	page := getPage(c)
	if page != 5 {
		t.Errorf("getPage = %d, want 5", page)
	}
}

func TestGetPage_Default(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	page := getPage(c)
	if page != 1 {
		t.Errorf("getPage default = %d, want 1", page)
	}
}

func TestGetPage_Negative(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test?page=-5", nil)

	page := getPage(c)
	if page != 1 {
		t.Errorf("getPage negative = %d, want 1", page)
	}
}

func TestGetPageSizePair(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test?page=3&page_size=30", nil)

	page, pageSize := getPageSizePair(c, 50)
	if page != 3 || pageSize != 30 {
		t.Errorf("getPageSizePair = (%d, %d), want (3, 30)", page, pageSize)
	}
}

func TestGetPageSizePair_Default(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	page, pageSize := getPageSizePair(c, 50)
	if page != 1 || pageSize != 50 {
		t.Errorf("getPageSizePair defaults = (%d, %d), want (1, 50)", page, pageSize)
	}
}

func TestGetPageSizePair_MaxClamp(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test?page_size=500", nil)

	_, pageSize := getPageSizePair(c, 50)
	if pageSize != 50 {
		t.Errorf("page_size should be clamped to 50, got %d", pageSize)
	}
}

func TestGetInt64Param(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "42"}}

	id := getInt64Param(c, "id")
	if id != 42 {
		t.Errorf("getInt64Param = %d, want 42", id)
	}
}

func TestGetInt64Param_Missing(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	id := getInt64Param(c, "nonexistent")
	if id != 0 {
		t.Errorf("getInt64Param missing = %d, want 0", id)
	}
}

func TestUserData_NoUser(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	data := userData(c, nil)
	if _, ok := data["user_id"]; ok {
		t.Error("user_id should not be set for unauthenticated request")
	}
}

func TestUserData_WithUserContext(t *testing.T) {
	// This test verifies that userData correctly extracts user context values.
	// We test with userID=0 to avoid DB dependency (userID>0 triggers FindByID).
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)
	c.Set("user_id", int64(0))
	c.Set("username", "")
	c.Set("role", "")

	data := userData(c, gin.H{"title": "Test"})
	if _, ok := data["user_id"]; ok {
		t.Error("user_id should not be set for userID=0")
	}
	if data["title"].(string) != "Test" {
		t.Errorf("title should be preserved: %v", data["title"])
	}
}

func TestUserData_WithCachedAvatar(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)
	// Simulate pre-cached avatar in context
	c.Set("user_id", int64(1))
	c.Set("username", "testuser")
	c.Set("role", "user")
	c.Set("ctx_avatar_url", "http://example.com/avatar.jpg")

	data := userData(c, gin.H{})
	if data["user_avatar"].(string) != "http://example.com/avatar.jpg" {
		t.Errorf("avatar = %v, want http://example.com/avatar.jpg", data["user_avatar"])
	}
}
