package upload

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"golang.org/x/image/webp"
)

const (
	ThumbSize         = 300
	MediumSize        = 1200
	MaxImagePixels    = 24_000_000
	MaxImageDimension = 12_000
)

// UploadResult holds URLs for all generated image sizes.
type UploadResult struct {
	UploadID string `json:"upload_id"`
	Original string `json:"original"`
	Medium   string `json:"medium"`
	Thumb    string `json:"thumb"`
}

// uploadSem limits concurrent image processing to prevent OOM.
var uploadSem = make(chan struct{}, 2)

var (
	ErrInvalidImage  = errors.New("invalid image")
	ErrImageTooLarge = errors.New("image too large")
)

// ProcessAndUpload reads an image, generates thumb+medium sizes,
// and uploads all three to MinIO. Returns URLs keyed by size.
func ProcessAndUpload(reader io.Reader, size int64, contentType, originalName string) (*UploadResult, error) {
	uploadSem <- struct{}{}
	defer func() { <-uploadSem }()
	// Read entire file into memory
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	if int64(len(data)) > maxSize {
		return nil, fmt.Errorf("%w: 文件过大，最大支持 %dMB", ErrImageTooLarge, maxSize/(1024*1024))
	}
	config, err := decodeImageConfig(bytes.NewReader(data), contentType)
	if err != nil {
		return nil, fmt.Errorf("%w: 图片内容无法解析", ErrInvalidImage)
	}
	if err := validateImageConfig(config); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrImageTooLarge, err)
	}

	_ = originalName
	ext := extensionForContentType(contentType)
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
		UploadID: id,
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
		mediumBuf, err := encodeJPEG(mediumImg, 90)
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

// ObjectKeyFromImageURL converts an internal image URL into a validated
// storage key. URLs are client input and must never be treated as arbitrary
// object names.
func ObjectKeyFromImageURL(raw string) (string, error) {
	const prefix = "/api/v1/images/"
	if !strings.HasPrefix(raw, prefix) {
		return "", fmt.Errorf("invalid image URL")
	}
	key := strings.TrimPrefix(raw, prefix)
	if !strings.HasPrefix(key, "posts/") || key == "posts/" ||
		strings.ContainsAny(key, "\\\x00?#") || strings.Contains(key, "..") {
		return "", fmt.Errorf("invalid image object key")
	}
	return key, nil
}

func ObjectKeys(result *UploadResult) []string {
	if result == nil {
		return nil
	}
	keys := make([]string, 0, 3)
	seen := make(map[string]struct{}, 3)
	for _, raw := range []string{result.Original, result.Medium, result.Thumb} {
		key, err := ObjectKeyFromImageURL(raw)
		if err != nil {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys
}

// BatchIDFromObjectKey extracts the UUID shared by the original, medium and
// thumbnail objects generated by one upload.
func BatchIDFromObjectKey(objectKey string) string {
	name := filepath.Base(objectKey)
	if strings.HasSuffix(name, ".jpg") {
		name = strings.TrimSuffix(name, ".jpg")
	} else {
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	return name
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

func decodeImageConfig(r io.Reader, contentType string) (image.Config, error) {
	switch contentType {
	case "image/jpeg":
		return jpeg.DecodeConfig(r)
	case "image/png":
		return png.DecodeConfig(r)
	case "image/webp":
		return webp.DecodeConfig(r)
	case "image/gif":
		return gif.DecodeConfig(r)
	default:
		return image.Config{}, fmt.Errorf("unsupported image type")
	}
}

func extensionForContentType(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".jpg"
	}
}

func validateImageConfig(config image.Config) error {
	if config.Width <= 0 || config.Height <= 0 {
		return fmt.Errorf("%w: 图片尺寸无效", ErrInvalidImage)
	}
	if config.Width > MaxImageDimension || config.Height > MaxImageDimension {
		return fmt.Errorf("%w: 图片尺寸过大，单边最大支持%d像素", ErrImageTooLarge, MaxImageDimension)
	}
	if int64(config.Width)*int64(config.Height) > MaxImagePixels {
		return fmt.Errorf("%w: 图片像素过多，最多支持%d百万像素", ErrImageTooLarge, MaxImagePixels/1_000_000)
	}
	return nil
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
