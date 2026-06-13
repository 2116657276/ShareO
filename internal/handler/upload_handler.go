package handler

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/pkg/upload"
)

type UploadHandler struct{}

func NewUploadHandler() *UploadHandler { return &UploadHandler{} }

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
		response.InternalError(c, "图片上传失败，请稍后重试")
		return
	}

	response.Success(c, gin.H{
		"url":  result.Medium, // default: medium for backward compatibility
		"urls": result,
	})
}

func (h *UploadHandler) ServeImage(c *gin.Context) {
	objectName := strings.TrimPrefix(c.Param("objectName"), "/")

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
