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
	"zerogravity-82/goph-keeper/internal/storage/postgres/dto"
	"zerogravity-82/goph-keeper/internal/usecase"
)

// TestRecordRepository_CreateAndGetByIDAndUserID проверяет создание и получение приватной записи без файла.
func TestRecordRepository_CreateAndGetByIDAndUserID(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	userRepo, err := NewUserRepository(db)
	require.NoError(t, err)
	recordRepo, err := NewRecordRepository(db)
	require.NoError(t, err)
	user := newTestUser(t, "record-user")
	record := newTestRecord(t, user.ID, model.RecordTypeCredential, "credential title", fixedTestTime())

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

// TestRecordRepository_GetByIDAndUserID_WithFile проверяет получение приватной записи вместе с файлом.
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
	record := newTestRecord(t, user.ID, model.RecordTypeBinary, "binary title", fixedTestTime())
	file := newTestRecordFile(t, record.ID, fixedTestTime())

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

// TestRecordRepository_GetByIDAndUserID_NotFound проверяет ошибку при поиске отсутствующей или чужой приватной записи.
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

// TestRecordRepository_ListByUserID проверяет получение списка приватных записей.
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
	now := fixedTestTime()
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

// TestRecordRepository_Update проверяет обновление приватной записи с увеличением версии.
func TestRecordRepository_Update(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	userRepo, err := NewUserRepository(db)
	require.NoError(t, err)
	recordRepo, err := NewRecordRepository(db)
	require.NoError(t, err)
	user := newTestUser(t, "record-update-user")
	record := newTestRecord(t, user.ID, model.RecordTypeText, "old title", fixedTestTime())
	updatedAt := record.UpdatedAt.Add(time.Minute).UTC().Truncate(time.Microsecond)

	require.NoError(t, userRepo.Create(ctx, user))
	require.NoError(t, recordRepo.Create(ctx, record))

	record.Title = "new title"
	record.Description = "new description"
	record.EncryptedDEK = model.EncryptedBlob{Data: []byte("new-encrypted-dek")}
	record.EncryptedPayload = model.EncryptedBlob{Data: []byte("new-encrypted-payload")}
	record.UpdatedAt = updatedAt

	// Act
	version, err := recordRepo.Update(ctx, record, 1)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, int64(2), version)

	got, err := recordRepo.GetByIDAndUserID(ctx, record.ID, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "new title", got.Title)
	assert.Equal(t, "new description", got.Description)
	assert.Equal(t, []byte("new-encrypted-dek"), got.EncryptedDEK.Data)
	assert.Equal(t, []byte("new-encrypted-payload"), got.EncryptedPayload.Data)
	assert.Equal(t, int64(2), got.Version)
	assert.True(t, got.CreatedAt.Equal(record.CreatedAt))
	assert.True(t, got.UpdatedAt.Equal(updatedAt))
}

// TestRecordRepository_Update_NotFound проверяет ошибку при обновлении отсутствующей, чужой или удаленной приватной
// записи.
func TestRecordRepository_Update_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	userRepo, err := NewUserRepository(db)
	require.NoError(t, err)
	recordRepo, err := NewRecordRepository(db)
	require.NoError(t, err)
	user := newTestUser(t, "record-update-not-found-user")
	otherUser := newTestUser(t, "record-update-not-found-other-user")
	now := fixedTestTime()
	deletedRecord := newTestRecord(t, user.ID, model.RecordTypeText, "deleted title", now)
	deletedAt := now.Add(time.Minute)
	deletedRecord.DeletedAt = &deletedAt

	require.NoError(t, userRepo.Create(ctx, user))
	require.NoError(t, userRepo.Create(ctx, otherUser))
	require.NoError(t, recordRepo.Create(ctx, deletedRecord))

	tests := []struct {
		name   string
		record model.Record
	}{
		{name: "missing record", record: newTestRecord(t, user.ID, model.RecordTypeText, "missing title", now)},
		{name: "other user", record: model.Record{ID: deletedRecord.ID, UserID: otherUser.ID}},
		{name: "deleted record", record: model.Record{ID: deletedRecord.ID, UserID: user.ID}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			record := tt.record
			record.Title = "new title"
			record.Description = "new description"
			record.EncryptedDEK = model.EncryptedBlob{Data: []byte("new-dek")}
			record.EncryptedPayload = model.EncryptedBlob{Data: []byte("new-payload")}
			record.UpdatedAt = now.Add(time.Minute)

			// Act
			_, err := recordRepo.Update(ctx, record, 1)

			// Assert
			require.Error(t, err)
			assert.True(t, errors.Is(err, usecase.ErrRecordNotFound))
		})
	}
}

// TestRecordRepository_Update_VersionConflict проверяет ошибку при несовпадении ожидаемой версии приватной записи.
func TestRecordRepository_Update_VersionConflict(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	userRepo, err := NewUserRepository(db)
	require.NoError(t, err)
	recordRepo, err := NewRecordRepository(db)
	require.NoError(t, err)
	user := newTestUser(t, "record-update-version-conflict-user")
	record := newTestRecord(t, user.ID, model.RecordTypeText, "title", fixedTestTime())
	originalTitle := record.Title
	originalVersion := record.Version

	require.NoError(t, userRepo.Create(ctx, user))
	require.NoError(t, recordRepo.Create(ctx, record))

	record.Title = "new title"
	record.UpdatedAt = fixedTestTime().Add(time.Minute)

	// Act
	_, err = recordRepo.Update(ctx, record, 999)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, usecase.ErrRecordVersionConflict))

	got, err := recordRepo.GetByIDAndUserID(ctx, record.ID, user.ID)
	require.NoError(t, err)
	assert.Equal(t, originalTitle, got.Title)
	assert.Equal(t, originalVersion, got.Version)
}

// TestRecordRepository_ReencryptDEKs проверяет обновление зашифрованных DEK всех активных записей пользователя.
func TestRecordRepository_ReencryptDEKs(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	userRepo, err := NewUserRepository(db)
	require.NoError(t, err)
	recordRepo, err := NewRecordRepository(db)
	require.NoError(t, err)
	user := newTestUser(t, "record-reencrypt-deks-user")
	otherUser := newTestUser(t, "record-reencrypt-deks-other-user")
	now := fixedTestTime()
	firstRecord := newTestRecord(t, user.ID, model.RecordTypeText, "первая запись", now)
	secondRecord := newTestRecord(t, user.ID, model.RecordTypeCredential, "вторая запись", now.Add(time.Minute))
	deletedRecord := newTestRecord(t, user.ID, model.RecordTypeCard, "удаленная запись", now.Add(2*time.Minute))
	deletedAt := now.Add(3 * time.Minute)
	deletedRecord.DeletedAt = &deletedAt
	otherRecord := newTestRecord(t, otherUser.ID, model.RecordTypeText, "еще одна запись", now)
	updatedAt := now.Add(4 * time.Minute)

	require.NoError(t, userRepo.Create(ctx, user))
	require.NoError(t, userRepo.Create(ctx, otherUser))
	require.NoError(t, recordRepo.Create(ctx, firstRecord))
	require.NoError(t, recordRepo.Create(ctx, secondRecord))
	require.NoError(t, recordRepo.Create(ctx, deletedRecord))
	require.NoError(t, recordRepo.Create(ctx, otherRecord))

	// Act
	err = recordRepo.ReencryptDEKs(
		ctx,
		user.ID,
		[]usecase.ReencryptedRecordDEK{
			{
				RecordID:        firstRecord.ID,
				ExpectedVersion: firstRecord.Version,
				EncryptedDEK:    []byte("first-new-encrypted-dek"),
			},
			{
				RecordID:        secondRecord.ID,
				ExpectedVersion: secondRecord.Version,
				EncryptedDEK:    []byte("second-new-encrypted-dek"),
			},
		},
		updatedAt,
	)

	// Assert
	require.NoError(t, err)

	gotFirst, err := recordRepo.GetByIDAndUserID(ctx, firstRecord.ID, user.ID)
	require.NoError(t, err)
	assert.Equal(t, []byte("first-new-encrypted-dek"), gotFirst.EncryptedDEK.Data)
	assert.Equal(t, firstRecord.Version+1, gotFirst.Version)
	assert.Equal(t, firstRecord.Title, gotFirst.Title)
	assert.Equal(t, firstRecord.EncryptedPayload.Data, gotFirst.EncryptedPayload.Data)
	assert.True(t, gotFirst.UpdatedAt.Equal(updatedAt))

	gotSecond, err := recordRepo.GetByIDAndUserID(ctx, secondRecord.ID, user.ID)
	require.NoError(t, err)
	assert.Equal(t, []byte("second-new-encrypted-dek"), gotSecond.EncryptedDEK.Data)
	assert.Equal(t, secondRecord.Version+1, gotSecond.Version)
	assert.Equal(t, secondRecord.Title, gotSecond.Title)
	assert.Equal(t, secondRecord.EncryptedPayload.Data, gotSecond.EncryptedPayload.Data)
	assert.True(t, gotSecond.UpdatedAt.Equal(updatedAt))

	gotOther, err := recordRepo.GetByIDAndUserID(ctx, otherRecord.ID, otherUser.ID)
	require.NoError(t, err)
	assertRecordEqual(t, otherRecord, gotOther)
}

// TestRecordRepository_ReencryptDEKs_VersionConflict проверяет конфликт при смене мастер-ключа, если версия записи
// изменилась после чтения клиентом.
func TestRecordRepository_ReencryptDEKs_VersionConflict(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	userRepo, err := NewUserRepository(db)
	require.NoError(t, err)
	recordRepo, err := NewRecordRepository(db)
	require.NoError(t, err)
	user := newTestUser(t, "record-reencrypt-deks-conflict-user")
	record := newTestRecord(t, user.ID, model.RecordTypeText, "просто запись", fixedTestTime())

	require.NoError(t, userRepo.Create(ctx, user))
	require.NoError(t, recordRepo.Create(ctx, record))

	// Act
	err = recordRepo.ReencryptDEKs(
		ctx,
		user.ID,
		[]usecase.ReencryptedRecordDEK{
			{
				RecordID:        record.ID,
				ExpectedVersion: record.Version + 1,
				EncryptedDEK:    []byte("new-encrypted-dek"),
			},
		},
		fixedTestTime().Add(time.Minute),
	)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, usecase.ErrMasterKeyChangeConflict))

	got, err := recordRepo.GetByIDAndUserID(ctx, record.ID, user.ID)
	require.NoError(t, err)
	assertRecordEqual(t, record, got)
}

// TestRecordMutationGuard_BlocksSameUser проверяет, что Advisory Lock сериализует операции одного пользователя и не
// блокирует операции другого пользователя.
func TestRecordMutationGuard_BlocksSameUser(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	guard, err := NewRecordMutationGuard(db)
	require.NoError(t, err)
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000051")
	otherUserID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000052")
	lockHeld := make(chan struct{})
	releaseLock := make(chan struct{})
	firstTxDone := make(chan error, 1)

	go func() {
		firstTxDone <- guard.WithUserRecordsLock(ctx, userID, func(context.Context) error {
			close(lockHeld)
			<-releaseLock
			return nil
		})
	}()

	select {
	case <-lockHeld:
	case err := <-firstTxDone:
		require.NoError(t, err)
	case <-time.After(time.Second):
		require.FailNow(t, "lock was not acquired in time")
	}
	defer func() {
		close(releaseLock)
		assert.NoError(t, <-firstTxDone)
	}()

	// Act: другой пользователь не блокируется.
	otherUserCtx, cancelOtherUser := context.WithTimeout(ctx, time.Second)
	err = guard.WithUserRecordsLock(otherUserCtx, otherUserID, func(context.Context) error {
		return nil
	})
	cancelOtherUser()

	// Assert
	require.NoError(t, err)

	// Act: тот же пользователь блокируется до освобождения первой транзакции.
	sameUserCtx, cancelSameUser := context.WithTimeout(ctx, 200*time.Millisecond)
	err = guard.WithUserRecordsLock(sameUserCtx, userID, func(context.Context) error {
		return nil
	})
	cancelSameUser()

	// Assert
	require.Error(t, err)
}

// TestRecordRepository_Delete проверяет мягкое удаление приватной записи.
func TestRecordRepository_Delete(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	userRepo, err := NewUserRepository(db)
	require.NoError(t, err)
	recordRepo, err := NewRecordRepository(db)
	require.NoError(t, err)
	user := newTestUser(t, "record-delete-user")
	record := newTestRecord(t, user.ID, model.RecordTypeText, "delete title", fixedTestTime())
	deletedAt := fixedTestTime().Add(time.Minute)

	require.NoError(t, userRepo.Create(ctx, user))
	require.NoError(t, recordRepo.Create(ctx, record))

	// Act
	err = recordRepo.Delete(ctx, record.ID, user.ID, deletedAt)

	// Assert
	require.NoError(t, err)

	var stored dto.Record
	err = db.GetContext(ctx, &stored, `
SELECT id, app_user_id, type, title, description, encrypted_dek, encrypted_payload, version, created_at, updated_at, deleted_at
FROM record
WHERE id = $1
`, record.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.DeletedAt)
	assert.True(t, stored.DeletedAt.Equal(deletedAt))
	assert.True(t, stored.UpdatedAt.Equal(deletedAt))

	_, err = recordRepo.GetByIDAndUserID(ctx, record.ID, user.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, usecase.ErrRecordNotFound))

	items, err := recordRepo.ListByUserID(ctx, user.ID)
	require.NoError(t, err)
	assert.Empty(t, items)
}

// TestRecordRepository_Delete_NotFound проверяет ошибку при удалении отсутствующей, чужой или уже удаленной записи.
func TestRecordRepository_Delete_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	userRepo, err := NewUserRepository(db)
	require.NoError(t, err)
	recordRepo, err := NewRecordRepository(db)
	require.NoError(t, err)
	user := newTestUser(t, "record-delete-not-found-user")
	otherUser := newTestUser(t, "record-delete-not-found-other-user")
	now := fixedTestTime()
	record := newTestRecord(t, user.ID, model.RecordTypeText, "delete title", now)
	deletedRecord := newTestRecord(t, user.ID, model.RecordTypeText, "already deleted title", now)
	deletedAt := now.Add(time.Minute)
	deletedRecord.DeletedAt = &deletedAt

	require.NoError(t, userRepo.Create(ctx, user))
	require.NoError(t, userRepo.Create(ctx, otherUser))
	require.NoError(t, recordRepo.Create(ctx, record))
	require.NoError(t, recordRepo.Create(ctx, deletedRecord))

	tests := []struct {
		name     string
		recordID uuid.UUID
		userID   uuid.UUID
	}{
		{name: "missing record", recordID: uuid.New(), userID: user.ID},
		{name: "other user", recordID: record.ID, userID: otherUser.ID},
		{name: "already deleted", recordID: deletedRecord.ID, userID: user.ID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			err := recordRepo.Delete(ctx, tt.recordID, tt.userID, now.Add(time.Minute))

			// Assert
			require.Error(t, err)
			assert.True(t, errors.Is(err, usecase.ErrRecordNotFound))
		})
	}
}

// TestRecordFileRepository_UpdateUploadStatus проверяет обновление статуса загрузки файла приватной записи.
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
	record := newTestRecord(t, user.ID, model.RecordTypeBinary, "binary title", fixedTestTime())
	file := newTestRecordFile(t, record.ID, fixedTestTime())
	updatedAt := fixedTestTime().Add(time.Minute)

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
	err = fileRepo.UpdateUploadStatus(ctx, uuid.New(), model.UploadStatusUploaded, fixedTestTime())

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, usecase.ErrRecordNotFound))
}

// TestMultipartUploadRepository_CreateGetPartsAndUpdateStatus проверяет жизненный цикл состояния multipart-загрузки.
func TestMultipartUploadRepository_CreateGetPartsAndUpdateStatus(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	userRepo, err := NewUserRepository(db)
	require.NoError(t, err)
	recordRepo, err := NewRecordRepository(db)
	require.NoError(t, err)
	fileRepo, err := NewRecordFileRepository(db)
	require.NoError(t, err)
	multipartRepo, err := NewMultipartUploadRepository(db)
	require.NoError(t, err)
	user := newTestUser(t, "multipart-upload-user")
	record := newTestRecord(t, user.ID, model.RecordTypeBinary, "multipart-загрузка", fixedTestTime())
	file := newTestRecordFile(t, record.ID, fixedTestTime())
	upload := usecase.MultipartUpload{
		ID:              uuid.MustParse("018f6b7c-0000-7000-8000-200000000001"),
		UserID:          user.ID,
		RecordID:        record.ID,
		RecordVersion:   record.Version,
		FileID:          file.ID,
		ObjectKey:       "users/user/records/record/files/file/payload",
		StorageUploadID: "storage-upload-id",
		EncryptedSize:   15,
		PartSize:        5,
		Status:          usecase.MultipartUploadStatusUploading,
		CreatedAt:       fixedTestTime(),
		UpdatedAt:       fixedTestTime(),
	}
	firstPart := usecase.MultipartUploadPart{
		UploadID:   upload.ID,
		PartNumber: 2,
		Size:       5,
		ETag:       "etag-2",
		CreatedAt:  fixedTestTime().Add(time.Minute),
	}
	secondPart := usecase.MultipartUploadPart{
		UploadID:   upload.ID,
		PartNumber: 1,
		Size:       5,
		ETag:       "etag-1",
		CreatedAt:  fixedTestTime().Add(2 * time.Minute),
	}
	completedAt := fixedTestTime().Add(3 * time.Minute)

	require.NoError(t, userRepo.Create(ctx, user))
	require.NoError(t, recordRepo.Create(ctx, record))
	require.NoError(t, fileRepo.Create(ctx, file))

	// Act
	err = multipartRepo.Create(ctx, upload)

	// Assert
	require.NoError(t, err)
	got, err := multipartRepo.GetByIDAndUserID(ctx, upload.ID, user.ID)
	require.NoError(t, err)
	assertMultipartUploadEqual(t, upload, got)

	require.NoError(t, multipartRepo.UpsertPart(ctx, firstPart))
	require.NoError(t, multipartRepo.UpsertPart(ctx, secondPart))

	parts, err := multipartRepo.ListParts(ctx, upload.ID)
	require.NoError(t, err)
	require.Len(t, parts, 2)
	assertMultipartUploadPartEqual(t, secondPart, parts[0])
	assertMultipartUploadPartEqual(t, firstPart, parts[1])

	firstPart.Size = 4
	firstPart.ETag = "etag-2-retry"
	firstPart.CreatedAt = fixedTestTime().Add(4 * time.Minute)
	require.NoError(t, multipartRepo.UpsertPart(ctx, firstPart))

	parts, err = multipartRepo.ListParts(ctx, upload.ID)
	require.NoError(t, err)
	require.Len(t, parts, 2)
	assertMultipartUploadPartEqual(t, firstPart, parts[1])

	err = multipartRepo.UpdateStatus(ctx, upload.ID, usecase.MultipartUploadStatusCompleted, completedAt, &completedAt)
	require.NoError(t, err)
	got, err = multipartRepo.GetByIDAndUserID(ctx, upload.ID, user.ID)
	require.NoError(t, err)
	assert.Equal(t, usecase.MultipartUploadStatusCompleted, got.Status)
	assert.True(t, got.UpdatedAt.Equal(completedAt))
	require.NotNil(t, got.CompletedAt)
	assert.True(t, got.CompletedAt.Equal(completedAt))
}

// TestMultipartUploadRepository_NotFound проверяет ошибки поиска и обновления отсутствующей multipart-загрузки.
func TestMultipartUploadRepository_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	multipartRepo, err := NewMultipartUploadRepository(db)
	require.NoError(t, err)
	missingUploadID := uuid.MustParse("018f6b7c-0000-7000-8000-200000000002")

	// Act
	_, getErr := multipartRepo.GetByIDAndUserID(ctx, missingUploadID, uuid.New())
	updateErr := multipartRepo.UpdateStatus(
		ctx,
		missingUploadID,
		usecase.MultipartUploadStatusAborted,
		fixedTestTime(),
		nil,
	)

	// Assert
	require.Error(t, getErr)
	assert.True(t, errors.Is(getErr, usecase.ErrMultipartUploadNotFound))
	require.Error(t, updateErr)
	assert.True(t, errors.Is(updateErr, usecase.ErrMultipartUploadNotFound))
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

	return model.RecordFile{
		ID:            id,
		RecordID:      recordID,
		ObjectKey:     "users/user-id/records/record-id/files/file-id/payload",
		EncryptedSize: &encryptedSize,
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
	assert.Equal(t, expected.UploadStatus, actual.UploadStatus)
	assert.True(t, actual.CreatedAt.Equal(expected.CreatedAt))
	assert.True(t, actual.UpdatedAt.Equal(expected.UpdatedAt))
}

func assertMultipartUploadEqual(t *testing.T, expected, actual usecase.MultipartUpload) {
	t.Helper()

	assert.Equal(t, expected.ID, actual.ID)
	assert.Equal(t, expected.UserID, actual.UserID)
	assert.Equal(t, expected.RecordID, actual.RecordID)
	assert.Equal(t, expected.RecordVersion, actual.RecordVersion)
	assert.Equal(t, expected.FileID, actual.FileID)
	assert.Equal(t, expected.ObjectKey, actual.ObjectKey)
	assert.Equal(t, expected.StorageUploadID, actual.StorageUploadID)
	assert.Equal(t, expected.EncryptedSize, actual.EncryptedSize)
	assert.Equal(t, expected.PartSize, actual.PartSize)
	assert.Equal(t, expected.Status, actual.Status)
	assert.True(t, actual.CreatedAt.Equal(expected.CreatedAt))
	assert.True(t, actual.UpdatedAt.Equal(expected.UpdatedAt))
	assert.Equal(t, expected.CompletedAt, actual.CompletedAt)
}

func assertMultipartUploadPartEqual(t *testing.T, expected, actual usecase.MultipartUploadPart) {
	t.Helper()

	assert.Equal(t, expected.UploadID, actual.UploadID)
	assert.Equal(t, expected.PartNumber, actual.PartNumber)
	assert.Equal(t, expected.Size, actual.Size)
	assert.Equal(t, expected.ETag, actual.ETag)
	assert.True(t, actual.CreatedAt.Equal(expected.CreatedAt))
}
