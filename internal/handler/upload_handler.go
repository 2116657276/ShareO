package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
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
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "文件过大，最大支持50MB"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请选择文件"})
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	allowed := map[string]bool{
		"image/jpeg": true, "image/png": true,
		"image/webp": true, "image/gif": true,
	}
	if !allowed[contentType] {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "不支持的图片格式，仅支持 JPG/PNG/WebP/GIF"})
		return
	}

	if header.Size <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "文件为空"})
		return
	}
	if header.Size > upload.MaxUploadSize() {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "文件过大，最大支持50MB"})
		return
	}

	url, err := upload.UploadImage(file, header.Size, contentType, header.Filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "上传失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"url": url},
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
		c.Header("Cache-Control", "public, max-age=86400")
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
	c.Header("Cache-Control", "public, max-age=86400")
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
