package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/pkg/upload"
	"github.com/zhoujianlin/ShareO/internal/repository"
)

type UploadHandler struct {
	postRepo     *repository.PostRepo
	artifactRepo uploadArtifactStore
}

type uploadArtifactStore interface {
	Register(context.Context, int64, string, []string, time.Time) error
}

func NewUploadHandler(postRepo *repository.PostRepo, artifactRepos ...uploadArtifactStore) *UploadHandler {
	repo := repository.NewPostRepo()
	if postRepo != nil {
		repo = postRepo
	}
	var artifactRepo uploadArtifactStore = repository.NewUploadArtifactRepo()
	if len(artifactRepos) > 0 && artifactRepos[0] != nil {
		artifactRepo = artifactRepos[0]
	}
	return &UploadHandler{postRepo: repo, artifactRepo: artifactRepo}
}

func (h *UploadHandler) UploadImage(c *gin.Context) {
	// Enforce max body size (50MB + overhead)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, upload.MaxUploadSize()+10<<20)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			response.BadRequest(c, "文件过大，最大支持50MB")
			return
		}
		response.BadRequest(c, "请选择文件")
		return
	}
	defer file.Close()

	// Check file size from header
	if header.Size <= 0 {
		response.BadRequest(c, "文件为空")
		return
	}
	if header.Size > upload.MaxUploadSize() {
		response.BadRequest(c, "文件过大，最大支持50MB")
		return
	}

	// Magic number detection: read first 512 bytes to detect real content type
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		response.BadRequest(c, "无法读取文件")
		return
	}
	detectedType := http.DetectContentType(buf[:n])

	// http.DetectContentType does not support WebP—detect manually
	if detectedType == "application/octet-stream" && n >= 12 &&
		string(buf[0:4]) == "RIFF" && string(buf[8:12]) == "WEBP" {
		detectedType = "image/webp"
	}

	allowed := map[string]bool{
		"image/jpeg": true, "image/png": true,
		"image/webp": true, "image/gif": true,
	}
	if !allowed[detectedType] {
		response.BadRequest(c, "不支持的图片格式，仅支持 JPG/PNG/WebP/GIF")
		return
	}

	// Prepend already-read bytes back — safe regardless of Seek support
	reader := io.MultiReader(bytes.NewReader(buf[:n]), file)

	result, err := upload.ProcessAndUpload(reader, header.Size, detectedType, header.Filename)
	if err != nil {
		if errors.Is(err, upload.ErrInvalidImage) || errors.Is(err, upload.ErrImageTooLarge) {
			response.BadRequest(c, "图片内容无效或尺寸过大")
			return
		}
		response.InternalError(c, "图片上传失败，请稍后重试")
		return
	}
	objectKeys := upload.ObjectKeys(result)
	if err := h.artifactRepo.Register(c.Request.Context(), c.GetInt64("user_id"), result.UploadID, objectKeys, time.Now().Add(24*time.Hour)); err != nil {
		if removeErr := upload.RemoveObjects(c.Request.Context(), objectKeys); removeErr != nil {
			slog.Warn("failed to remove unregistered upload objects", "user_id", c.GetInt64("user_id"), "err", removeErr)
		}
		slog.Warn("failed to register upload artifacts", "user_id", c.GetInt64("user_id"), "err", err)
		response.InternalError(c, "图片上传暂时无法完成，请稍后重试")
		return
	}

	response.Success(c, gin.H{
		"url":  result.Medium, // default: medium for backward compatibility
		"urls": result,
	})
}

func (h *UploadHandler) ServeImage(c *gin.Context) {
	objectName := strings.TrimPrefix(c.Param("objectName"), "/")
	if !validImageObjectName(objectName) {
		c.Status(http.StatusNotFound)
		return
	}
	visible, err := h.postRepo.IsApprovedImage(c.Request.Context(), objectName)
	if err != nil {
		slog.Warn("image visibility lookup failed", "object_name", objectName, "err", err)
		c.Status(http.StatusServiceUnavailable)
		return
	}
	if !visible {
		c.Status(http.StatusNotFound)
		return
	}

	// HEAD request: just return headers
	if c.Request.Method == http.MethodHead {
		info, err := upload.StatImage(objectName)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("Content-Type", info.ContentType)
		c.Header("Content-Length", fmt.Sprintf("%d", info.Size))
		c.Header("Cache-Control", "public, max-age=86400, immutable")
		c.Status(http.StatusOK)
		return
	}

	obj, contentType, err := upload.GetImage(objectName)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer obj.Close()

	c.Header("Content-Type", contentType)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "public, max-age=86400, immutable")
	c.Stream(func(w io.Writer) bool {
		buf := make([]byte, 32*1024)
		for {
			n, err := obj.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
			}
			if err != nil {
				return false
			}
		}
	})
}

func validImageObjectName(objectName string) bool {
	return strings.HasPrefix(objectName, "posts/") &&
		!strings.ContainsAny(objectName, "\\\x00?#") &&
		!strings.Contains(objectName, "..")
}
