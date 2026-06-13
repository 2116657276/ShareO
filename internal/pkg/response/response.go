package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Business error codes — decoupled from HTTP status codes.
// API consumers should check `code == 0` for success.
const (
	ErrCodeSuccess      = 0
	ErrCodeBadRequest   = 1001
	ErrCodeUnauthorized = 1002
	ErrCodeForbidden    = 1003
	ErrCodeNotFound     = 1004
	ErrCodeInternal     = 1005
	ErrCodeRateLimit    = 1006
)

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    ErrCodeSuccess,
		Message: "success",
		Data:    data,
	})
}

func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    ErrCodeSuccess,
		Message: message,
		Data:    data,
	})
}

func Error(c *gin.Context, httpStatus int, bizCode int, message string) {
	c.JSON(httpStatus, Response{
		Code:    bizCode,
		Message: message,
	})
}

func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, ErrCodeBadRequest, message)
}

func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, ErrCodeUnauthorized, message)
}

func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, ErrCodeForbidden, message)
}

func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, ErrCodeNotFound, message)
}

func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, ErrCodeInternal, message)
}

// PageResponse for paginated API responses
type PageResponse struct {
	List       interface{} `json:"list"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}
