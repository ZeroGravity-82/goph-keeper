//go:build integration

package minio

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMinIOStorage_PutAndGet_Integration проверяет реальную запись и чтение объекта через тестовый MinIO.
func TestMinIOStorage_PutAndGet_Integration(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := newIntegrationMinIOStorage(t, ctx)
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-000000000001")
	recordID := uuid.MustParse("018f6b7c-0000-7000-8000-000000000002")
	fileID := uuid.MustParse("018f6b7c-0000-7000-8000-000000000003")
	objectKey := storage.ObjectKey(userID, recordID, fileID)
	data := []byte("encrypted file payload")

	// Act
	written, err := storage.Put(ctx, objectKey, bytes.NewReader(data), int64(len(data)))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, int64(len(data)), written)

	// Act
	reader, err := storage.Get(ctx, objectKey)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, reader.Close())
	}()
	got, err := io.ReadAll(reader)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func newIntegrationMinIOStorage(t *testing.T, ctx context.Context) *MinIOStorage {
	t.Helper()

	endpoint := os.Getenv("TEST_FILE_STORAGE_ENDPOINT")
	if endpoint == "" {
		t.Skip("TEST_FILE_STORAGE_ENDPOINT is not set")
	}
	accessKey := os.Getenv("TEST_FILE_STORAGE_ACCESS_KEY")
	secretKey := os.Getenv("TEST_FILE_STORAGE_SECRET_KEY")
	bucket := os.Getenv("TEST_FILE_STORAGE_BUCKET")
	require.NotEmpty(t, accessKey)
	require.NotEmpty(t, secretKey)
	require.NotEmpty(t, bucket)

	var (
		storage *MinIOStorage
		err     error
	)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		storage, err = NewMinIOStorage(ctx, endpoint, accessKey, secretKey, bucket, false)
		if err == nil {
			return storage
		}
		time.Sleep(500 * time.Millisecond)
	}

	require.NoError(t, err)
	return storage
}
