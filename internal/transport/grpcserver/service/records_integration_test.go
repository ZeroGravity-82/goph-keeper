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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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

func getStoredRecord(t *testing.T, ctx context.Context, db *sqlx.DB, recordID uuid.UUID) dto.Record {
	t.Helper()

	var record dto.Record
	err := db.GetContext(ctx, &record, `
SELECT
    id,
    app_user_id,
    type,
    title,
    description,
    encrypted_dek,
    encrypted_payload,
    version,
    created_at,
    updated_at,
    deleted_at
FROM record
WHERE id = $1
`, recordID)
	require.NoError(t, err)
	return record
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

// TestRecordsService_ListRecords_Integration проверяет получение списка приватных записей пользователя.
func TestRecordsService_ListRecords_Integration(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	user := createIntegrationUser(t, ctx, db, "record-list-user")
	otherUser := createIntegrationUser(t, ctx, db, "record-list-other-user")
	recordsService := newIntegrationRecordsService(t, db)
	requestCtx := authcontext.WithUserID(ctx, user.ID)
	otherUserCtx := authcontext.WithUserID(ctx, otherUser.ID)

	textRecordID := createIntegrationRecord(
		t,
		requestCtx,
		recordsService,
		pb.RecordType_RECORD_TYPE_TEXT,
		"text title",
	)
	binaryRecordID := createIntegrationRecord(
		t,
		requestCtx,
		recordsService,
		pb.RecordType_RECORD_TYPE_BINARY,
		"binary title",
	)
	deletedRecordID := createIntegrationRecord(
		t,
		requestCtx,
		recordsService,
		pb.RecordType_RECORD_TYPE_CARD,
		"deleted title",
	)
	_ = createIntegrationRecord(t, otherUserCtx, recordsService, pb.RecordType_RECORD_TYPE_TEXT, "other user title")

	textUpdatedAt := time.Date(2026, time.June, 23, 10, 0, 0, 0, time.UTC)
	binaryUpdatedAt := textUpdatedAt.Add(time.Hour)
	deletedAt := binaryUpdatedAt.Add(time.Hour)
	_, err := db.ExecContext(ctx, `UPDATE record SET updated_at = $1 WHERE id = $2`, textUpdatedAt, textRecordID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE record SET updated_at = $1 WHERE id = $2`, binaryUpdatedAt, binaryRecordID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE record SET deleted_at = $1 WHERE id = $2`, deletedAt, deletedRecordID)
	require.NoError(t, err)

	// Act
	resp, err := recordsService.ListRecords(requestCtx, pb.ListRecordsRequest_builder{}.Build())

	// Assert
	require.NoError(t, err)
	require.Len(t, resp.GetItems(), 2)

	first := resp.GetItems()[0]
	assert.Equal(t, binaryRecordID.String(), first.GetRecordId())
	assert.Equal(t, pb.RecordType_RECORD_TYPE_BINARY, first.GetType())
	assert.Equal(t, "binary title", first.GetTitle())
	assert.Equal(t, binaryUpdatedAt, first.GetUpdatedAt().AsTime())
	require.NotNil(t, first.GetFile())
	assert.Equal(t, pb.UploadStatus_UPLOAD_STATUS_PENDING, first.GetFile().GetUploadStatus())

	second := resp.GetItems()[1]
	assert.Equal(t, textRecordID.String(), second.GetRecordId())
	assert.Equal(t, pb.RecordType_RECORD_TYPE_TEXT, second.GetType())
	assert.Equal(t, "text title", second.GetTitle())
	assert.Equal(t, textUpdatedAt, second.GetUpdatedAt().AsTime())
	assert.Nil(t, second.GetFile())
}

func createIntegrationRecord(
	t *testing.T,
	ctx context.Context,
	recordsService *RecordsService,
	recordType pb.RecordType,
	title string,
) uuid.UUID {
	t.Helper()

	req := pb.CreateRecordRequest_builder{
		Type:             &recordType,
		Title:            &title,
		Description:      new("description"),
		EncryptedDek:     []byte("encrypted-dek"),
		EncryptedPayload: []byte("encrypted-payload"),
	}.Build()
	resp, err := recordsService.CreateRecord(ctx, req)
	require.NoError(t, err)
	recordID, err := uuid.Parse(resp.GetRecordId())
	require.NoError(t, err)
	return recordID
}

// TestRecordsService_GetRecord_Integration_Binary проверяет получение бинарной записи с зашифрованными данными и
// статусом файла.
func TestRecordsService_GetRecord_Integration_Binary(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	user := createIntegrationUser(t, ctx, db, "record-get-binary-user")
	recordsService := newIntegrationRecordsService(t, db)
	requestCtx := authcontext.WithUserID(ctx, user.ID)
	recordID := createIntegrationRecord(t, requestCtx, recordsService, pb.RecordType_RECORD_TYPE_BINARY, "binary title")
	req := pb.GetRecordRequest_builder{RecordId: new(recordID.String())}.Build()

	// Act
	resp, err := recordsService.GetRecord(requestCtx, req)

	// Assert
	require.NoError(t, err)
	record := resp.GetRecord()
	require.NotNil(t, record)
	assert.Equal(t, recordID.String(), record.GetRecordId())
	assert.Equal(t, pb.RecordType_RECORD_TYPE_BINARY, record.GetType())
	assert.Equal(t, "binary title", record.GetTitle())
	assert.Equal(t, "description", record.GetDescription())
	assert.Equal(t, []byte("encrypted-dek"), record.GetEncryptedDek())
	assert.Equal(t, []byte("encrypted-payload"), record.GetEncryptedPayload())
	assert.Equal(t, int64(1), record.GetVersion())
	assert.NotNil(t, record.GetCreatedAt())
	assert.NotNil(t, record.GetUpdatedAt())
	assert.Nil(t, record.GetDeletedAt())
	require.NotNil(t, record.GetFile())
	assert.Equal(t, pb.UploadStatus_UPLOAD_STATUS_PENDING, record.GetFile().GetUploadStatus())
}

// TestRecordsService_GetRecord_Integration_NotFoundForOtherUser проверяет, что пользователь не может получить чужую
// приватную запись.
func TestRecordsService_GetRecord_Integration_NotFoundForOtherUser(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	owner := createIntegrationUser(t, ctx, db, "record-get-owner")
	otherUser := createIntegrationUser(t, ctx, db, "record-get-other-user")
	recordsService := newIntegrationRecordsService(t, db)
	ownerCtx := authcontext.WithUserID(ctx, owner.ID)
	otherUserCtx := authcontext.WithUserID(ctx, otherUser.ID)
	recordID := createIntegrationRecord(t, ownerCtx, recordsService, pb.RecordType_RECORD_TYPE_TEXT, "text title")
	req := pb.GetRecordRequest_builder{RecordId: new(recordID.String())}.Build()

	// Act
	_, err := recordsService.GetRecord(otherUserCtx, req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// TestRecordsService_GetRecord_Integration_NotFoundForDeletedRecord проверяет, что помеченная как удаленная запись не
// отдается пользователю.
func TestRecordsService_GetRecord_Integration_NotFoundForDeletedRecord(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	user := createIntegrationUser(t, ctx, db, "record-get-deleted-user")
	recordsService := newIntegrationRecordsService(t, db)
	requestCtx := authcontext.WithUserID(ctx, user.ID)
	recordID := createIntegrationRecord(t, requestCtx, recordsService, pb.RecordType_RECORD_TYPE_TEXT, "deleted title")
	_, err := db.ExecContext(ctx, `UPDATE record SET deleted_at = $1 WHERE id = $2`, time.Now().UTC(), recordID)
	require.NoError(t, err)
	req := pb.GetRecordRequest_builder{RecordId: new(recordID.String())}.Build()

	// Act
	_, err = recordsService.GetRecord(requestCtx, req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}
