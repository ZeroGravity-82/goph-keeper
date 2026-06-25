//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/goph-keeper/internal/domain/model"
	"zerogravity-82/goph-keeper/internal/usecase"
)

// TestRecordRepository_CreateAndGetByIDAndUserID проверяет создание и получение записи пользователя без файла.
func TestRecordRepository_CreateAndGetByIDAndUserID(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	userRepo, err := NewUserRepository(db)
	require.NoError(t, err)
	recordRepo, err := NewRecordRepository(db)
	require.NoError(t, err)
	user := newTestUser(t, "record-user")
	record := newTestRecord(t, user.ID, model.RecordTypeCredential, "credential title", time.Now().UTC())

	require.NoError(t, userRepo.Create(ctx, user))

	// Act
	err = recordRepo.Create(ctx, record)

	// Assert
	require.NoError(t, err)

	got, err := recordRepo.GetByIDAndUserID(ctx, record.ID, user.ID)
	require.NoError(t, err)
	assertRecordEqual(t, record, got)
	assert.Nil(t, got.File)
}

// TestRecordRepository_GetByIDAndUserID_WithFile проверяет получение записи пользователя вместе с файлом.
func TestRecordRepository_GetByIDAndUserID_WithFile(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	userRepo, err := NewUserRepository(db)
	require.NoError(t, err)
	recordRepo, err := NewRecordRepository(db)
	require.NoError(t, err)
	fileRepo, err := NewRecordFileRepository(db)
	require.NoError(t, err)
	user := newTestUser(t, "record-file-user")
	record := newTestRecord(t, user.ID, model.RecordTypeBinary, "binary title", time.Now().UTC())
	file := newTestRecordFile(t, record.ID, time.Now().UTC())

	require.NoError(t, userRepo.Create(ctx, user))
	require.NoError(t, recordRepo.Create(ctx, record))
	require.NoError(t, fileRepo.Create(ctx, file))

	// Act
	got, err := recordRepo.GetByIDAndUserID(ctx, record.ID, user.ID)

	// Assert
	require.NoError(t, err)
	assertRecordEqual(t, record, got)
	require.NotNil(t, got.File)
	assertRecordFileEqual(t, file, *got.File)
}

// TestRecordRepository_GetByIDAndUserID_NotFound проверяет ошибку при поиске отсутствующей или чужой записи.
func TestRecordRepository_GetByIDAndUserID_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	recordRepo, err := NewRecordRepository(db)
	require.NoError(t, err)

	// Act
	_, err = recordRepo.GetByIDAndUserID(ctx, uuid.New(), uuid.New())

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, usecase.ErrRecordNotFound))
}

// TestRecordRepository_ListByUserID проверяет получение списка записей пользователя.
func TestRecordRepository_ListByUserID(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	userRepo, err := NewUserRepository(db)
	require.NoError(t, err)
	recordRepo, err := NewRecordRepository(db)
	require.NoError(t, err)
	fileRepo, err := NewRecordFileRepository(db)
	require.NoError(t, err)
	user := newTestUser(t, "record-list-user")
	otherUser := newTestUser(t, "record-list-other-user")
	now := time.Now().UTC()
	olderRecord := newTestRecord(t, user.ID, model.RecordTypeText, "older title", now.Add(-time.Hour))
	newerRecord := newTestRecord(t, user.ID, model.RecordTypeBinary, "newer title", now)
	deletedAt := now.Add(time.Minute)
	deletedRecord := newTestRecord(t, user.ID, model.RecordTypeCard, "deleted title", now.Add(time.Hour))
	deletedRecord.DeletedAt = &deletedAt
	otherRecord := newTestRecord(t, otherUser.ID, model.RecordTypeText, "other title", now.Add(2*time.Hour))
	file := newTestRecordFile(t, newerRecord.ID, now)

	require.NoError(t, userRepo.Create(ctx, user))
	require.NoError(t, userRepo.Create(ctx, otherUser))
	require.NoError(t, recordRepo.Create(ctx, olderRecord))
	require.NoError(t, recordRepo.Create(ctx, newerRecord))
	require.NoError(t, recordRepo.Create(ctx, deletedRecord))
	require.NoError(t, recordRepo.Create(ctx, otherRecord))
	require.NoError(t, fileRepo.Create(ctx, file))

	// Act
	items, err := recordRepo.ListByUserID(ctx, user.ID)

	// Assert
	require.NoError(t, err)
	require.Len(t, items, 2)

	assert.Equal(t, newerRecord.ID, items[0].ID)
	assert.Equal(t, newerRecord.Type, items[0].Type)
	assert.Equal(t, newerRecord.Title, items[0].Title)
	assert.Equal(t, newerRecord.Description, items[0].Description)
	assert.True(t, items[0].CreatedAt.Equal(newerRecord.CreatedAt))
	assert.True(t, items[0].UpdatedAt.Equal(newerRecord.UpdatedAt))
	require.NotNil(t, items[0].File)
	assert.Equal(t, file.UploadStatus, items[0].File.UploadStatus)

	assert.Equal(t, olderRecord.ID, items[1].ID)
	assert.Equal(t, olderRecord.Type, items[1].Type)
	assert.Nil(t, items[1].File)
}

// TestRecordFileRepository_UpdateUploadStatus проверяет обновление статуса загрузки файла записи пользователя.
func TestRecordFileRepository_UpdateUploadStatus(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	userRepo, err := NewUserRepository(db)
	require.NoError(t, err)
	recordRepo, err := NewRecordRepository(db)
	require.NoError(t, err)
	fileRepo, err := NewRecordFileRepository(db)
	require.NoError(t, err)
	user := newTestUser(t, "record-file-status-user")
	record := newTestRecord(t, user.ID, model.RecordTypeBinary, "binary title", time.Now().UTC())
	file := newTestRecordFile(t, record.ID, time.Now().UTC())
	updatedAt := time.Now().UTC().Add(time.Minute).Truncate(time.Microsecond)

	require.NoError(t, userRepo.Create(ctx, user))
	require.NoError(t, recordRepo.Create(ctx, record))
	require.NoError(t, fileRepo.Create(ctx, file))

	// Act
	err = fileRepo.UpdateUploadStatus(ctx, file.ID, model.UploadStatusFailed, updatedAt)

	// Assert
	require.NoError(t, err)
	got, err := recordRepo.GetByIDAndUserID(ctx, record.ID, user.ID)
	require.NoError(t, err)
	require.NotNil(t, got.File)
	assert.Equal(t, model.UploadStatusFailed, got.File.UploadStatus)
	assert.True(t, got.File.UpdatedAt.Equal(updatedAt))
}

// TestRecordFileRepository_UpdateUploadStatus_NotFound проверяет ошибку при обновлении статуса отсутствующего файла.
func TestRecordFileRepository_UpdateUploadStatus_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	fileRepo, err := NewRecordFileRepository(db)
	require.NoError(t, err)

	// Act
	err = fileRepo.UpdateUploadStatus(ctx, uuid.New(), model.UploadStatusUploaded, time.Now().UTC())

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, usecase.ErrRecordNotFound))
}

func newTestRecord(
	t *testing.T,
	userID uuid.UUID,
	recordType model.RecordType,
	title string,
	now time.Time,
) model.Record {
	t.Helper()

	id, err := uuid.NewV7()
	require.NoError(t, err)
	now = now.UTC().Truncate(time.Microsecond)

	return model.Record{
		ID:               id,
		UserID:           userID,
		Type:             recordType,
		Title:            title,
		Description:      "description",
		EncryptedDEK:     model.EncryptedBlob{Data: []byte("encrypted-dek")},
		EncryptedPayload: model.EncryptedBlob{Data: []byte("encrypted-payload")},
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
		DeletedAt:        nil,
	}
}

func newTestRecordFile(t *testing.T, recordID uuid.UUID, now time.Time) model.RecordFile {
	t.Helper()

	id, err := uuid.NewV7()
	require.NoError(t, err)
	now = now.UTC().Truncate(time.Microsecond)
	encryptedSize := int64(1024)
	uploadMode := model.UploadModeSinglePart

	return model.RecordFile{
		ID:            id,
		RecordID:      recordID,
		ObjectKey:     "users/user-id/records/record-id/files/file-id/payload",
		EncryptedSize: &encryptedSize,
		UploadMode:    &uploadMode,
		UploadStatus:  model.UploadStatusUploaded,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func assertRecordEqual(t *testing.T, expected, actual model.Record) {
	t.Helper()

	assert.Equal(t, expected.ID, actual.ID)
	assert.Equal(t, expected.UserID, actual.UserID)
	assert.Equal(t, expected.Type, actual.Type)
	assert.Equal(t, expected.Title, actual.Title)
	assert.Equal(t, expected.Description, actual.Description)
	assert.Equal(t, expected.EncryptedDEK.Data, actual.EncryptedDEK.Data)
	assert.Equal(t, expected.EncryptedPayload.Data, actual.EncryptedPayload.Data)
	assert.Equal(t, expected.Version, actual.Version)
	assert.True(t, actual.CreatedAt.Equal(expected.CreatedAt))
	assert.True(t, actual.UpdatedAt.Equal(expected.UpdatedAt))
	assert.Equal(t, expected.DeletedAt, actual.DeletedAt)
}

func assertRecordFileEqual(t *testing.T, expected, actual model.RecordFile) {
	t.Helper()

	assert.Equal(t, expected.ID, actual.ID)
	assert.Equal(t, expected.RecordID, actual.RecordID)
	assert.Equal(t, expected.ObjectKey, actual.ObjectKey)
	require.NotNil(t, actual.EncryptedSize)
	assert.Equal(t, *expected.EncryptedSize, *actual.EncryptedSize)
	require.NotNil(t, actual.UploadMode)
	assert.Equal(t, *expected.UploadMode, *actual.UploadMode)
	assert.Equal(t, expected.UploadStatus, actual.UploadStatus)
	assert.True(t, actual.CreatedAt.Equal(expected.CreatedAt))
	assert.True(t, actual.UpdatedAt.Equal(expected.UpdatedAt))
}
