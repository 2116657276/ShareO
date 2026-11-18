package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestBodyLimitRejectsOversizedNonUploadRequest(t *testing.T) {
	r := gin.New()
	r.Use(RequestBodyLimit(32, "/api/v1/upload"))
	r.POST("/api/v1/posts", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts", bytes.NewReader(bytes.Repeat([]byte("x"), 64)))
	r.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestRequestBodyLimitSkipsUploadEndpoint(t *testing.T) {
	r := gin.New()
	r.Use(RequestBodyLimit(32, "/api/v1/upload"))
	r.POST("/api/v1/upload", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", bytes.NewReader(bytes.Repeat([]byte("x"), 64)))
	r.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want upload endpoint to remain available", recorder.Code)
	}
}
