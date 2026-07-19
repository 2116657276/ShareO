package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func TestSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	Success(c, map[string]string{"key": "val"})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 || resp.Message != "success" {
		t.Errorf("got code=%d msg=%s", resp.Code, resp.Message)
	}
}

func TestBadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	BadRequest(c, "bad input")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != ErrCodeBadRequest || resp.Message != "bad input" {
		t.Errorf("got code=%d msg=%s", resp.Code, resp.Message)
	}
}

func TestUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	Unauthorized(c, "请先登录")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != ErrCodeUnauthorized {
		t.Errorf("business code = %d, want %d", resp.Code, ErrCodeUnauthorized)
	}
	if resp.Message != "请先登录" {
		t.Errorf("message = %q, want %q", resp.Message, "请先登录")
	}
}

func TestForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	Forbidden(c, "需要管理员权限")

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != ErrCodeForbidden {
		t.Errorf("business code = %d, want %d", resp.Code, ErrCodeForbidden)
	}
	if resp.Message != "需要管理员权限" {
		t.Errorf("message = %q, want %q", resp.Message, "需要管理员权限")
	}
}

func TestNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	NotFound(c, "帖子不存在")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != ErrCodeNotFound {
		t.Errorf("business code = %d, want %d", resp.Code, ErrCodeNotFound)
	}
	if resp.Message != "帖子不存在" {
		t.Errorf("message = %q, want %q", resp.Message, "帖子不存在")
	}
}

func TestInternalError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	InternalError(c, "internal error")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != ErrCodeInternal {
		t.Errorf("business code = %d, want %d", resp.Code, ErrCodeInternal)
	}
	if resp.Message != "internal error" {
		t.Errorf("message = %q, want %q", resp.Message, "internal error")
	}
}

func TestSuccessWithMessage(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	SuccessWithMessage(c, "custom success msg", map[string]string{"key": "val"})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("business code = %d, want 0", resp.Code)
	}
	if resp.Message != "custom success msg" {
		t.Errorf("message = %q, want %q", resp.Message, "custom success msg")
	}
	if resp.Data == nil {
		t.Error("data should not be nil")
	}
}

func TestError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	Error(c, http.StatusBadRequest, ErrCodeBadRequest, "direct error call")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != ErrCodeBadRequest {
		t.Errorf("business code = %d, want %d", resp.Code, ErrCodeBadRequest)
	}
	if resp.Message != "direct error call" {
		t.Errorf("message = %q, want %q", resp.Message, "direct error call")
	}
}

func TestPageResponse(t *testing.T) {
	pr := PageResponse{
		List:      []int{1, 2, 3},
		Total:     100,
		Page:      1,
		PageSize:  20,
		TotalPages: 5,
	}
	b, _ := json.Marshal(pr)
	var out PageResponse
	json.Unmarshal(b, &out)
	if out.Total != 100 || out.Page != 1 || out.TotalPages != 5 {
		t.Errorf("PageResponse marshal/unmarshal mismatch: got total=%d page=%d", out.Total, out.Page)
	}
}
