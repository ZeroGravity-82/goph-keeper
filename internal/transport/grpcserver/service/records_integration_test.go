//go:build integration

package service

import (
	"context"
	"fmt"
	"io"
	"sync"
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
	recordUC, err := usecase.NewRecordUseCase(recordRepo, recordFileRepo, newFakeFileStorage(), transactor)
	require.NoError(t, err)
	recordsService, err := NewRecordsService(recordUC, logging.NopLogger())
	require.NoError(t, err)
	return recordsService
}

type fakeFileStorage struct {
	mu      sync.Mutex
	objects map[string][]byte
	err     error
}

func newFakeFileStorage() *fakeFileStorage {
	return &fakeFileStorage{objects: make(map[string][]byte)}
}

func (s *fakeFileStorage) ObjectKey(userID, recordID, fileID uuid.UUID) string {
	return fmt.Sprintf("users/%s/records/%s/files/%s/payload", userID, recordID, fileID)
}

func (s *fakeFileStorage) Put(_ context.Context, objectKey string, data io.Reader, _ int64) (int64, error) {
	if s.err != nil {
		return 0, s.err
	}
	copied, err := io.ReadAll(data)
	if err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.objects[objectKey] = copied
	return int64(len(copied)), nil
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

// TestRecordsService_CreateRecord_Integration_Binary проверяет, что бинарная запись не создается обычным методом.
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
	_, err := recordsService.CreateRecord(requestCtx, req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	var recordCount int
	err = db.GetContext(ctx, &recordCount, `SELECT COUNT(*) FROM record WHERE app_user_id = $1`, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, recordCount)
}

// TestRecordsService_CreateBinaryRecord_Integration проверяет создание бинарной записи через реальные БД-зависимости.
func TestRecordsService_CreateBinaryRecord_Integration(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	user := createIntegrationUser(t, ctx, db, "record-create-binary-user")
	recordsService := newIntegrationRecordsService(t, db)
	requestCtx := authcontext.WithUserID(ctx, user.ID)
	encryptedFile := []byte("encrypted-file")
	stream := newCreateBinaryRecordTestStream(
		requestCtx,
		newCreateBinaryRecordMetadata("binary title", int64(len(encryptedFile))),
		encryptedFile,
	)

	// Act
	err := recordsService.CreateBinaryRecord(stream)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, stream.response)
	assert.NotEmpty(t, stream.response.GetRecordId())
	assert.Equal(t, int64(1), stream.response.GetVersion())
	assert.Equal(t, pb.UploadStatus_UPLOAD_STATUS_UPLOADED, stream.response.GetUploadStatus())

	recordID, err := uuid.Parse(stream.response.GetRecordId())
	require.NoError(t, err)
	storedRecord := getStoredRecord(t, ctx, db, recordID)
	assert.Equal(t, user.ID, storedRecord.UserID)
	assert.Equal(t, string(model.RecordTypeBinary), storedRecord.Type)
	assert.Equal(t, "binary title", storedRecord.Title)
	assert.Equal(t, "description", storedRecord.Description)
	assert.Equal(t, []byte("encrypted-dek"), storedRecord.EncryptedDEK)
	assert.Equal(t, []byte("encrypted-payload"), storedRecord.EncryptedPayload)

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
	require.NotNil(t, storedFile.EncryptedSize)
	assert.Equal(t, int64(len(encryptedFile)), *storedFile.EncryptedSize)
	require.NotNil(t, storedFile.UploadMode)
	assert.Equal(t, string(model.UploadModeSinglePart), *storedFile.UploadMode)
	assert.Equal(t, string(model.UploadStatusUploaded), storedFile.UploadStatus)
}

// TestRecordsService_CreateBinaryRecord_Integration_SizeMismatch проверяет, что при несовпадении размера запись файла
// получает статус failed.
func TestRecordsService_CreateBinaryRecord_Integration_SizeMismatch(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	user := createIntegrationUser(t, ctx, db, "record-create-binary-size-mismatch-user")
	recordsService := newIntegrationRecordsService(t, db)
	requestCtx := authcontext.WithUserID(ctx, user.ID)
	stream := newCreateBinaryRecordTestStream(
		requestCtx,
		newCreateBinaryRecordMetadata("binary title", 100),
		[]byte("short"),
	)

	// Act
	err := recordsService.CreateBinaryRecord(stream)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	var storedFile dto.RecordFile
	err = db.GetContext(ctx, &storedFile, `
SELECT rf.id, rf.record_id, rf.object_key, rf.encrypted_size, rf.upload_mode, rf.upload_status, rf.created_at, rf.updated_at
FROM record_file rf
JOIN record r ON r.id = rf.record_id
WHERE r.app_user_id = $1
`, user.ID)
	require.NoError(t, err)
	assert.Equal(t, string(model.UploadStatusFailed), storedFile.UploadStatus)
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
	binaryRecordID := createIntegrationBinaryRecord(
		t,
		ctx,
		db,
		user.ID,
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
	assert.Equal(t, pb.UploadStatus_UPLOAD_STATUS_UPLOADED, first.GetFile().GetUploadStatus())

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

func createIntegrationBinaryRecord(
	t *testing.T,
	ctx context.Context,
	db *sqlx.DB,
	userID uuid.UUID,
	title string,
) uuid.UUID {
	t.Helper()

	recordID, err := uuid.NewV7()
	require.NoError(t, err)
	fileID, err := uuid.NewV7()
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Microsecond)

	_, err = db.ExecContext(ctx, `
INSERT INTO record (
    id, app_user_id, type, title, description, encrypted_dek, encrypted_payload, version, created_at, updated_at, deleted_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NULL)
`,
		recordID,
		userID,
		string(model.RecordTypeBinary),
		title,
		"description",
		[]byte("encrypted-dek"),
		[]byte("encrypted-payload"),
		int64(1),
		now,
		now,
	)
	require.NoError(t, err)

	encryptedSize := int64(1024)
	uploadMode := string(model.UploadModeSinglePart)
	objectKey := fmt.Sprintf("users/%s/records/%s/files/%s/payload", userID, recordID, fileID)
	_, err = db.ExecContext(ctx, `
INSERT INTO record_file (
    id, record_id, object_key, encrypted_size, upload_mode, upload_status, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
`,
		fileID,
		recordID,
		objectKey,
		encryptedSize,
		uploadMode,
		string(model.UploadStatusUploaded),
		now,
		now,
	)
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
	recordID := createIntegrationBinaryRecord(t, ctx, db, user.ID, "binary title")
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
	assert.Equal(t, pb.UploadStatus_UPLOAD_STATUS_UPLOADED, record.GetFile().GetUploadStatus())
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
