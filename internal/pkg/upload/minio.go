package upload

import (
	"context"
	"fmt"
	"io"

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

// RemoveObjects deletes a bounded set of generated upload objects. It is used
// when metadata registration fails or an expired unbound upload is collected.
func RemoveObjects(ctx context.Context, objectNames []string) error {
	if Client == nil || len(objectNames) == 0 {
		return nil
	}
	var firstErr error
	seen := make(map[string]struct{}, len(objectNames))
	for _, objectName := range objectNames {
		if objectName == "" {
			continue
		}
		if _, ok := seen[objectName]; ok {
			continue
		}
		seen[objectName] = struct{}{}
		if err := Client.RemoveObject(ctx, bucketName, objectName, minio.RemoveObjectOptions{}); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
