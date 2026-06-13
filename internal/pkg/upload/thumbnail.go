package upload

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"path/filepath"
	"time"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"golang.org/x/image/webp"
)

const (
	ThumbSize  = 300
	MediumSize = 1200
)

// UploadResult holds URLs for all generated image sizes.
type UploadResult struct {
	Original string `json:"original"`
	Medium   string `json:"medium"`
	Thumb    string `json:"thumb"`
}

// ProcessAndUpload reads an image, generates thumb+medium sizes,
// and uploads all three to MinIO. Returns URLs keyed by size.
func ProcessAndUpload(reader io.Reader, size int64, contentType, originalName string) (*UploadResult, error) {
	// Read entire file into memory
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	if int64(len(data)) > maxSize {
		return nil, fmt.Errorf("文件过大，最大支持 %dMB", maxSize/(1024*1024))
	}

	ext := filepath.Ext(originalName)
	if ext == "" {
		ext = ".jpg"
	}
	datePrefix := time.Now().Format("2006/01/02")
	id := uuid.New().String()

	// Upload original
	origName := fmt.Sprintf("posts/original/%s/%s%s", datePrefix, id, ext)
	info, err := Client.PutObject(context.Background(), bucketName, origName,
		bytes.NewReader(data), int64(len(data)),
		minioPutOpts(contentType))
	if err != nil {
		return nil, fmt.Errorf("上传原图失败: %w", err)
	}
	if info.Size <= 0 {
		Client.RemoveObject(context.Background(), bucketName, origName, minioRemoveOpts())
		return nil, fmt.Errorf("上传后文件为空，请重试")
	}

	result := &UploadResult{
		Original: "/api/v1/images/" + origName,
	}

	// Decode image for resizing
	srcImg, err := decodeImage(bytes.NewReader(data), contentType)
	if err != nil || srcImg == nil {
		// Can't decode — medium and thumb fall back to original URL
		result.Medium = result.Original
		result.Thumb = result.Original
		return result, nil
	}

	bounds := srcImg.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Generate thumb (square crop → fit 300x300)
	thumbImg := squareCrop(srcImg, w, h)
	thumbImg = imaging.Fit(thumbImg, ThumbSize, ThumbSize, imaging.Lanczos)
	thumbBuf, err := encodeJPEG(thumbImg, 82)
	if err != nil {
		result.Thumb = result.Original
	} else {
		thumbName := fmt.Sprintf("posts/thumb/%s/%s.jpg", datePrefix, id)
		_, err = Client.PutObject(context.Background(), bucketName, thumbName,
			bytes.NewReader(thumbBuf), int64(len(thumbBuf)),
			minioPutOpts("image/jpeg"))
		if err == nil {
			result.Thumb = "/api/v1/images/" + thumbName
		} else {
			result.Thumb = result.Original
		}
	}

	// Generate medium (fit within 1200, skip if smaller)
	if w <= MediumSize && h <= MediumSize {
		result.Medium = result.Original
	} else {
		mediumImg := imaging.Fit(srcImg, MediumSize, MediumSize, imaging.Lanczos)
		mediumBuf, err := encodeJPEG(mediumImg, 85)
		if err != nil {
			result.Medium = result.Original
		} else {
			mediumName := fmt.Sprintf("posts/medium/%s/%s.jpg", datePrefix, id)
			_, err = Client.PutObject(context.Background(), bucketName, mediumName,
				bytes.NewReader(mediumBuf), int64(len(mediumBuf)),
				minioPutOpts("image/jpeg"))
			if err == nil {
				result.Medium = "/api/v1/images/" + mediumName
			} else {
				result.Medium = result.Original
			}
		}
	}

	return result, nil
}

func squareCrop(img image.Image, w, h int) image.Image {
	if w == h {
		return img
	}
	size := w
	offX := 0
	offY := 0
	if w > h {
		size = h
		offX = (w - h) / 2
	} else {
		size = w
		offY = (h - w) / 2
	}
	return imaging.Crop(img, image.Rect(offX, offY, offX+size, offY+size))
}

func decodeImage(r io.Reader, contentType string) (image.Image, error) {
	switch contentType {
	case "image/jpeg":
		return jpeg.Decode(r)
	case "image/png":
		return png.Decode(r)
	case "image/webp":
		return webp.Decode(r)
	case "image/gif":
		return gif.Decode(r)
	default:
		return nil, nil
	}
}

func encodeJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func minioPutOpts(ct string) minio.PutObjectOptions {
	return minio.PutObjectOptions{ContentType: ct}
}

func minioRemoveOpts() minio.RemoveObjectOptions {
	return minio.RemoveObjectOptions{}
}
