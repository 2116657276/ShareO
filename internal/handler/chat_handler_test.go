package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/service"
)

func TestChatErrorHTTPMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		err  error
		want int
	}{
		{service.ErrChatInvalid, http.StatusBadRequest},
		{service.ErrChatForbidden, http.StatusForbidden},
		{service.ErrChatNotFound, http.StatusNotFound},
		{service.ErrChatConflict, http.StatusConflict},
		{errors.New("database detail must not escape"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		handleChatError(ctx, tt.err)
		if recorder.Code != tt.want {
			t.Fatalf("error %v returned %d, want %d", tt.err, recorder.Code, tt.want)
		}
		if tt.want == http.StatusInternalServerError && strings.Contains(recorder.Body.String(), "database detail") {
			t.Fatal("internal database error leaked to client")
		}
	}
}
