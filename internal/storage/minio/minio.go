// Package minio реализует S3-совместимое хранилище зашифрованных бинарных файлов.
package minio

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"zerogravity-82/goph-keeper/internal/usecase"
)

// MinIOStorage сохраняет бинарные объекты в S3-совместимое хранилище.
type MinIOStorage struct {
	client *minio.Client
	core   *minio.Core
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

	return &MinIOStorage{client: client, core: &minio.Core{Client: client}, bucket: bucket}, nil
}

// ObjectKey строит ключ объекта по идентификаторам пользователя, приватной записи и файла.
func (s *MinIOStorage) ObjectKey(userID, recordID, fileID uuid.UUID) string {
	return fmt.Sprintf("users/%s/records/%s/files/%s/payload", userID, recordID, fileID)
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

// CreateMultipartUpload открывает multipart-загрузку объекта и возвращает ID сессии в объектном хранилище.
func (s *MinIOStorage) CreateMultipartUpload(ctx context.Context, objectKey string) (string, error) {
	if objectKey == "" {
		return "", errors.New("object key is not provided")
	}

	uploadID, err := s.core.NewMultipartUpload(ctx, s.bucket, objectKey, minio.PutObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create multipart upload: %w", err)
	}
	return uploadID, nil
}

// PutMultipartPart загружает одну часть multipart-объекта.
func (s *MinIOStorage) PutMultipartPart(
	ctx context.Context,
	objectKey string,
	uploadID string,
	partNumber int32,
	data io.Reader,
	size int64,
) (usecase.MultipartUploadPart, error) {
	if objectKey == "" {
		return usecase.MultipartUploadPart{}, errors.New("object key is not provided")
	}
	if uploadID == "" {
		return usecase.MultipartUploadPart{}, errors.New("multipart upload id is not provided")
	}
	if partNumber <= 0 {
		return usecase.MultipartUploadPart{}, errors.New("multipart part number is invalid")
	}
	if data == nil {
		return usecase.MultipartUploadPart{}, errors.New("multipart part data reader is not provided")
	}
	if size <= 0 {
		return usecase.MultipartUploadPart{}, errors.New("multipart part size is invalid")
	}

	part, err := s.core.PutObjectPart(
		ctx,
		s.bucket,
		objectKey,
		uploadID,
		int(partNumber),
		data,
		size,
		minio.PutObjectPartOptions{},
	)
	if err != nil {
		return usecase.MultipartUploadPart{}, fmt.Errorf("failed to put multipart part: %w", err)
	}
	return usecase.MultipartUploadPart{
		PartNumber: int32(part.PartNumber),
		Size:       part.Size,
		ETag:       part.ETag,
	}, nil
}

// CompleteMultipartUpload завершает multipart-загрузку и собирает объект из загруженных частей.
func (s *MinIOStorage) CompleteMultipartUpload(
	ctx context.Context,
	objectKey string,
	uploadID string,
	parts []usecase.MultipartUploadPart,
) error {
	if objectKey == "" {
		return errors.New("object key is not provided")
	}
	if uploadID == "" {
		return errors.New("multipart upload id is not provided")
	}
	if len(parts) == 0 {
		return errors.New("multipart parts are not provided")
	}

	completeParts := make([]minio.CompletePart, 0, len(parts))
	for _, part := range parts {
		completeParts = append(completeParts, minio.CompletePart{
			PartNumber: int(part.PartNumber),
			ETag:       part.ETag,
		})
	}
	_, err := s.core.CompleteMultipartUpload(ctx, s.bucket, objectKey, uploadID, completeParts, minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}
	return nil
}

// AbortMultipartUpload отменяет незавершенную multipart-загрузку в объектном хранилище.
func (s *MinIOStorage) AbortMultipartUpload(ctx context.Context, objectKey string, uploadID string) error {
	if objectKey == "" {
		return errors.New("object key is not provided")
	}
	if uploadID == "" {
		return errors.New("multipart upload id is not provided")
	}

	if err := s.core.AbortMultipartUpload(ctx, s.bucket, objectKey, uploadID); err != nil {
		return fmt.Errorf("failed to abort multipart upload: %w", err)
	}
	return nil
}
