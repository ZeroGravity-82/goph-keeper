//go:build integration

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/goph-keeper/internal/domain/model"
	"zerogravity-82/goph-keeper/internal/logging"
	"zerogravity-82/goph-keeper/internal/pb"
	"zerogravity-82/goph-keeper/internal/storage/postgres"
	"zerogravity-82/goph-keeper/internal/storage/postgres/dto"
	"zerogravity-82/goph-keeper/internal/transport/grpcserver/authcontext"
	"zerogravity-82/goph-keeper/internal/usecase"
)

// TestRecordsService_CreateRecord_Integration_Text проверяет создание текстовой записи через реальные зависимости.
func TestRecordsService_CreateRecord_Integration_Text(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	user := createIntegrationUser(t, ctx, db, "record-text-user")
	recordsService := newIntegrationRecordsService(t, db)
	requestCtx := authcontext.WithUserID(ctx, user.ID)
	recordType := pb.RecordType_RECORD_TYPE_TEXT
	req := pb.CreateRecordRequest_builder{
		Type:             &recordType,
		Title:            new("text title"),
		Description:      new("text description"),
		EncryptedDek:     []byte("encrypted-dek"),
		EncryptedPayload: []byte("encrypted-payload"),
	}.Build()

	// Act
	resp, err := recordsService.CreateRecord(requestCtx, req)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, resp.GetRecordId())
	assert.Equal(t, int64(1), resp.GetVersion())

	recordID, err := uuid.Parse(resp.GetRecordId())
	require.NoError(t, err)
	storedRecord := getStoredRecord(t, ctx, db, recordID)
	assert.Equal(t, user.ID, storedRecord.UserID)
	assert.Equal(t, string(model.RecordTypeText), storedRecord.Type)
	assert.Equal(t, "text title", storedRecord.Title)
	assert.Equal(t, "text description", storedRecord.Description)
	assert.Equal(t, []byte("encrypted-dek"), storedRecord.EncryptedDEK)
	assert.Equal(t, []byte("encrypted-payload"), storedRecord.EncryptedPayload)
	assert.Equal(t, int64(1), storedRecord.Version)
	assert.Nil(t, storedRecord.DeletedAt)

	var fileCount int
	err = db.GetContext(ctx, &fileCount, `SELECT COUNT(*) FROM record_file WHERE record_id = $1`, recordID)
	require.NoError(t, err)
	assert.Equal(t, 0, fileCount)
}

// TestRecordsService_CreateRecord_Integration_Binary проверяет создание бинарной записи с пустыми атрибутами файла и
// статусом загрузки файла `pending`.
func TestRecordsService_CreateRecord_Integration_Binary(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	user := createIntegrationUser(t, ctx, db, "record-binary-user")
	recordsService := newIntegrationRecordsService(t, db)
	requestCtx := authcontext.WithUserID(ctx, user.ID)
	recordType := pb.RecordType_RECORD_TYPE_BINARY
	req := pb.CreateRecordRequest_builder{
		Type:             &recordType,
		Title:            new("binary title"),
		Description:      new(""),
		EncryptedDek:     []byte("encrypted-dek"),
		EncryptedPayload: []byte("encrypted-payload"),
	}.Build()

	// Act
	resp, err := recordsService.CreateRecord(requestCtx, req)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, resp.GetRecordId())
	assert.Equal(t, int64(1), resp.GetVersion())

	recordID, err := uuid.Parse(resp.GetRecordId())
	require.NoError(t, err)
	storedRecord := getStoredRecord(t, ctx, db, recordID)
	assert.Equal(t, user.ID, storedRecord.UserID)
	assert.Equal(t, string(model.RecordTypeBinary), storedRecord.Type)

	var storedFile dto.RecordFile
	err = db.GetContext(ctx, &storedFile, `
SELECT id, record_id, object_key, encrypted_size, upload_mode, upload_status, created_at, updated_at
FROM record_file
WHERE record_id = $1
`, recordID)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, storedFile.ID)
	assert.Equal(t, recordID, storedFile.RecordID)
	expectedObjectKey := fmt.Sprintf("users/%s/records/%s/files/%s/payload", user.ID, recordID, storedFile.ID)
	assert.Equal(t, expectedObjectKey, storedFile.ObjectKey)
	assert.Nil(t, storedFile.EncryptedSize)
	assert.Nil(t, storedFile.UploadMode)
	assert.Equal(t, string(model.UploadStatusPending), storedFile.UploadStatus)
}

func newIntegrationRecordsService(t *testing.T, db *sqlx.DB) *RecordsService {
	t.Helper()

	recordRepo, err := postgres.NewRecordRepository(db)
	require.NoError(t, err)
	recordFileRepo, err := postgres.NewRecordFileRepository(db)
	require.NoError(t, err)
	transactor, err := postgres.NewTransactor(db)
	require.NoError(t, err)
	recordUC, err := usecase.NewRecordUseCase(recordRepo, recordFileRepo, transactor)
	require.NoError(t, err)
	recordsService, err := NewRecordsService(recordUC, logging.NopLogger())
	require.NoError(t, err)
	return recordsService
}

func createIntegrationUser(t *testing.T, ctx context.Context, db *sqlx.DB, login string) model.User {
	t.Helper()

	userRepo, err := postgres.NewUserRepository(db)
	require.NoError(t, err)
	userID, err := uuid.NewV7()
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Microsecond)
	user := model.User{
		ID:            userID,
		Login:         login,
		PasswordHash:  "password-hash",
		MasterKeySalt: []byte("master-key-salt"),
		RegisteredAt:  now,
		UpdatedAt:     now,
	}
	require.NoError(t, userRepo.Create(ctx, user))
	return user
}

func getStoredRecord(t *testing.T, ctx context.Context, db *sqlx.DB, recordID uuid.UUID) dto.Record {
	t.Helper()

	var record dto.Record
	err := db.GetContext(ctx, &record, `
SELECT id, app_user_id, type, title, description, encrypted_dek, encrypted_payload, version, created_at, updated_at, deleted_at
FROM record
WHERE id = $1
`, recordID)
	require.NoError(t, err)
	return record
}
