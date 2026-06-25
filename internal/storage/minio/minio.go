package minio

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOStorage сохраняет бинарные объекты в S3-совместимое хранилище.
type MinIOStorage struct {
	client *minio.Client
	bucket string
}

// NewMinIOStorage создает MinIOStorage и проверяет наличие bucket.
func NewMinIOStorage(
	ctx context.Context,
	endpoint, accessKey, secretKey, bucket string,
	useSSL bool,
) (*MinIOStorage, error) {
	if endpoint == "" {
		return nil, errors.New("object storage endpoint is not provided")
	}
	if accessKey == "" {
		return nil, errors.New("object storage access key is not provided")
	}
	if secretKey == "" {
		return nil, errors.New("object storage secret key is not provided")
	}
	if bucket == "" {
		return nil, errors.New("object storage bucket is not provided")
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create object storage client: %w", err)
	}

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check object storage bucket: %w", err)
	}
	if !exists {
		if err = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("failed to create object storage bucket: %w", err)
		}
	}

	return &MinIOStorage{client: client, bucket: bucket}, nil
}

// ObjectKey строит ключ объекта по идентификаторам пользователя, приватной записи и файла.
func (s *MinIOStorage) ObjectKey(userID, recordID, fileID uuid.UUID) string {
	return fmt.Sprintf("users/%s/records/%s/files/%s/payload", userID, recordID, fileID)
}

// Put сохраняет объект и возвращает фактически записанный размер.
func (s *MinIOStorage) Put(ctx context.Context, objectKey string, data io.Reader, size int64) (int64, error) {
	if objectKey == "" {
		return 0, errors.New("object key is not provided")
	}
	if data == nil {
		return 0, errors.New("object data reader is not provided")
	}
	if size <= 0 {
		return 0, errors.New("object size is invalid")
	}

	info, err := s.client.PutObject(
		ctx,
		s.bucket,
		objectKey,
		data,
		size,
		minio.PutObjectOptions{},
	)
	if err != nil {
		return 0, fmt.Errorf("failed to put object: %w", err)
	}
	return info.Size, nil
}

// Get возвращает поток для чтения объекта из файлового хранилища.
func (s *MinIOStorage) Get(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	if objectKey == "" {
		return nil, errors.New("object key is not provided")
	}

	object, err := s.client.GetObject(ctx, s.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	return object, nil
}
