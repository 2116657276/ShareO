package upload

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/zhoujianlin/ShareO/internal/config"
)

var Client *minio.Client
var bucketName string
var maxSize int64 = 50 * 1024 * 1024 // 50MB

func Init(cfg config.MinIOConfig) error {
	var err error
	Client, err = minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return fmt.Errorf("failed to init minio: %w", err)
	}

	bucketName = cfg.Bucket
	ctx := context.Background()
	exists, err := Client.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to check bucket: %w", err)
	}
	if !exists {
		if err := Client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	fmt.Println("MinIO connected, bucket ready:", bucketName)
	return nil
}

func MaxUploadSize() int64 { return maxSize }

func UploadImage(reader io.Reader, size int64, contentType, originalName string) (string, error) {
	if size > maxSize {
		return "", fmt.Errorf("文件过大，最大支持 %dMB", maxSize/(1024*1024))
	}
	if size <= 0 {
		return "", fmt.Errorf("文件为空")
	}

	ext := filepath.Ext(originalName)
	if ext == "" {
		ext = ".jpg"
	}
	objectName := fmt.Sprintf("posts/%s/%s%s", time.Now().Format("2006/01/02"), uuid.New().String(), ext)

	info, err := Client.PutObject(context.Background(), bucketName, objectName, reader, size,
		minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", fmt.Errorf("上传失败: %w", err)
	}

	// Validate: ensure the uploaded object has actual content
	if info.Size <= 0 {
		Client.RemoveObject(context.Background(), bucketName, objectName, minio.RemoveObjectOptions{})
		return "", fmt.Errorf("上传后文件为空，请重试")
	}

	url := fmt.Sprintf("/api/v1/images/%s", objectName)
	return url, nil
}

func StatImage(objectName string) (*minio.ObjectInfo, error) {
	info, err := Client.StatObject(context.Background(), bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func GetImage(objectName string) (io.ReadCloser, string, error) {
	obj, err := Client.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", err
	}
	stat, err := obj.Stat()
	if err != nil {
		return nil, "", err
	}
	return obj, stat.ContentType, nil
}
