package minio

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewMinIOStorage_RequiresConfig проверяет валидацию обязательных параметров объектного хранилища до подключения.
func TestNewMinIOStorage_RequiresConfig(t *testing.T) {
	tests := []struct {
		name      string
		endpoint  string
		accessKey string
		secretKey string
		bucket    string
		wantErr   string
	}{
		{
			name:      "endpoint",
			accessKey: "access",
			secretKey: "secret",
			bucket:    "bucket",
			wantErr:   "object storage endpoint is not provided",
		},
		{
			name:      "access key",
			endpoint:  "localhost:9000",
			secretKey: "secret",
			bucket:    "bucket",
			wantErr:   "object storage access key is not provided",
		},
		{
			name:      "secret key",
			endpoint:  "localhost:9000",
			accessKey: "access",
			bucket:    "bucket",
			wantErr:   "object storage secret key is not provided",
		},
		{
			name:      "bucket",
			endpoint:  "localhost:9000",
			accessKey: "access",
			secretKey: "secret",
			wantErr:   "object storage bucket is not provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			_, err := NewMinIOStorage(
				context.Background(),
				tt.endpoint,
				tt.accessKey,
				tt.secretKey,
				tt.bucket,
				false,
			)

			// Assert
			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())
		})
	}
}

// TestMinIOStorage_ObjectKey проверяет стабильный формат ключа объекта.
func TestMinIOStorage_ObjectKey(t *testing.T) {
	// Arrange
	storage := &MinIOStorage{}
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-000000000001")
	recordID := uuid.MustParse("018f6b7c-0000-7000-8000-000000000002")
	fileID := uuid.MustParse("018f6b7c-0000-7000-8000-000000000003")

	// Act
	key := storage.ObjectKey(userID, recordID, fileID)

	// Assert
	assert.Equal(
		t,
		"users/018f6b7c-0000-7000-8000-000000000001/"+
			"records/018f6b7c-0000-7000-8000-000000000002/"+
			"files/018f6b7c-0000-7000-8000-000000000003/payload",
		key,
	)
}

// TestMinIOStorage_GetRequiresObjectKey проверяет локальную валидацию ключа объекта перед чтением.
func TestMinIOStorage_GetRequiresObjectKey(t *testing.T) {
	// Arrange
	storage := &MinIOStorage{}

	// Act
	_, err := storage.Get(context.Background(), "")

	// Assert
	require.Error(t, err)
	assert.Equal(t, "object key is not provided", err.Error())
}
