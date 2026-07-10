package usecase

import (
	"bytes"
	"context"
	stdsha256 "crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/goph-keeper/internal/domain/model"
)

func testEncryptedSHA256(data []byte) string {
	sum := stdsha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// TestRecordUseCase_CreateRecord проверяет создание обычной приватной записи под guard-блокировкой пользователя.
func TestRecordUseCase_CreateRecord(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _, guard := newTestRecordUseCase(t)
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000001")

	// Act
	out, err := uc.CreateRecord(context.Background(), CreateRecordInput{
		UserID:           userID,
		Type:             model.RecordTypeCredential,
		Title:            "title",
		Description:      "description",
		EncryptedDEK:     []byte("encrypted-dek"),
		EncryptedPayload: []byte("encrypted-payload"),
	})

	// Assert
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, out.RecordID)
	assert.Equal(t, int64(1), out.Version)
	require.Len(t, recordRepo.created, 1)
	created := recordRepo.created[0]
	assert.Equal(t, out.RecordID, created.ID)
	assert.Equal(t, userID, created.UserID)
	assert.Equal(t, model.RecordTypeCredential, created.Type)
	assert.Equal(t, "title", created.Title)
	assert.Equal(t, "description", created.Description)
	assert.Equal(t, []byte("encrypted-dek"), created.EncryptedDEK.Data)
	assert.Equal(t, []byte("encrypted-payload"), created.EncryptedPayload.Data)
	assert.Equal(t, int64(1), created.Version)
	assert.Nil(t, created.DeletedAt)
	assert.Equal(t, 1, guard.calls)
	assert.Equal(t, []uuid.UUID{userID}, guard.lockedUserIDs)
	assert.True(t, guard.lockedInTx[0])
	assert.True(t, recordRepo.createdInTx[0])
}

// TestRecordUseCase_CreateRecord_FailWithBinaryType проверяет запрет создания бинарной записи обычным сценарием.
func TestRecordUseCase_CreateRecord_FailWithBinaryType(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _, _ := newTestRecordUseCase(t)

	// Act
	_, err := uc.CreateRecord(context.Background(), CreateRecordInput{Type: model.RecordTypeBinary})

	// Assert
	require.ErrorIs(t, err, ErrBinaryRecordNotSupported)
	assert.Empty(t, recordRepo.created)
}

// TestRecordUseCase_CreateRecord_FailWithRepositoryError проверяет ошибку сохранения приватной записи.
func TestRecordUseCase_CreateRecord_FailWithRepositoryError(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _, _ := newTestRecordUseCase(t)
	recordRepo.createErr = errTest

	// Act
	_, err := uc.CreateRecord(context.Background(), CreateRecordInput{Type: model.RecordTypeText})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestRecordUseCase_StartBinaryMultipartUpload проверяет создание записи, файла и серверной сессии multipart-загрузки
// под guard-блокировкой пользователя.
func TestRecordUseCase_StartBinaryMultipartUpload(t *testing.T) {
	// Arrange
	uc, recordRepo, recordFileRepo, multipartRepo, storage, _, guard := newTestRecordUseCase(t)
	storage.objectKey = "users/user/records/record/files/file/payload"
	storage.storageUploadID = "storage-upload-id-1"
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000021")

	// Act
	out, err := uc.StartBinaryMultipartUpload(context.Background(), StartBinaryMultipartUploadInput{
		UserID:           userID,
		Title:            "binary",
		Description:      "description",
		EncryptedDEK:     []byte("encrypted-dek"),
		EncryptedPayload: []byte("encrypted-payload"),
		EncryptedSize:    10 * 1024 * 1024,
		PartSize:         5 * 1024 * 1024,
	})

	// Assert
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, out.UploadID)
	assert.NotEqual(t, uuid.Nil, out.RecordID)
	assert.Equal(t, int64(1), out.Version)
	assert.Equal(t, int64(5*1024*1024), out.PartSize)
	assert.Equal(t, model.UploadStatusUploading, out.UploadStatus)
	assert.Equal(t, 1, guard.calls)
	assert.Equal(t, []uuid.UUID{userID}, guard.lockedUserIDs)
	assert.True(t, guard.lockedInTx[0])
	assert.Equal(t, "users/user/records/record/files/file/payload", storage.createMultipartKey)
	require.Len(t, recordRepo.created, 1)
	require.Len(t, recordFileRepo.created, 1)
	require.Len(t, multipartRepo.created, 1)
	assert.Equal(t, out.UploadID, multipartRepo.created[0].ID)
	assert.Equal(t, out.RecordID, multipartRepo.created[0].RecordID)
	assert.Equal(t, "storage-upload-id-1", multipartRepo.created[0].StorageUploadID)
	assert.Equal(t, MultipartUploadStatusUploading, multipartRepo.created[0].Status)
	assert.Equal(t, model.UploadStatusUploading, recordFileRepo.created[0].UploadStatus)
}

// TestRecordUseCase_StartBinaryMultipartUpload_FailWithRepositoryErrorAbortsStorageUpload проверяет отмену
// сессии multipart-загрузки в объектном хранилище, если состояние не удалось сохранить в БД.
func TestRecordUseCase_StartBinaryMultipartUpload_FailWithRepositoryErrorAbortsStorageUpload(t *testing.T) {
	// Arrange
	uc, _, _, multipartRepo, storage, _, _ := newTestRecordUseCase(t)
	multipartRepo.createErr = errTest
	storage.objectKey = "object-key"
	storage.storageUploadID = "storage-upload-id-2"

	// Act
	_, err := uc.StartBinaryMultipartUpload(context.Background(), StartBinaryMultipartUploadInput{
		EncryptedSize: 10,
		PartSize:      5,
	})

	// Assert
	require.ErrorIs(t, err, errTest)
	assert.Equal(t, 1, storage.abortMultipartCallCnt)
	assert.Equal(t, "object-key", storage.abortMultipartKey)
	assert.Equal(t, "storage-upload-id-2", storage.abortStorageUploadID)
}

// TestRecordUseCase_GetBinaryMultipartUploadStatus проверяет восстановление состояния уже загруженных частей.
func TestRecordUseCase_GetBinaryMultipartUploadStatus(t *testing.T) {
	// Arrange
	uc, _, _, multipartRepo, _, _, _ := newTestRecordUseCase(t)
	uploadID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000022")
	recordID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000023")
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000024")
	multipartRepo.upload = MultipartUpload{
		ID:            uploadID,
		UserID:        userID,
		RecordID:      recordID,
		EncryptedSize: 10,
		PartSize:      5,
		Status:        MultipartUploadStatusUploading,
	}
	multipartRepo.parts = []MultipartUploadPart{
		{UploadID: uploadID, PartNumber: 1, Size: 5, ETag: "etag-1"},
	}

	// Act
	out, err := uc.GetBinaryMultipartUploadStatus(context.Background(), GetBinaryMultipartUploadStatusInput{
		UserID:   userID,
		UploadID: uploadID,
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, uploadID, out.UploadID)
	assert.Equal(t, recordID, out.RecordID)
	assert.Equal(t, int64(10), out.EncryptedSize)
	assert.Equal(t, int64(5), out.PartSize)
	assert.Equal(t, model.UploadStatusUploading, out.UploadStatus)
	require.Len(t, out.UploadedParts, 1)
	assert.Equal(t, int32(1), out.UploadedParts[0].PartNumber)
	assert.Equal(t, int64(5), out.UploadedParts[0].Size)
	assert.Equal(t, "etag-1", out.UploadedParts[0].ETag)
}

// TestRecordUseCase_UploadBinaryMultipartPart проверяет загрузку одной части активной multipart-загрузки.
func TestRecordUseCase_UploadBinaryMultipartPart(t *testing.T) {
	// Arrange
	uc, _, _, multipartRepo, storage, _, _ := newTestRecordUseCase(t)
	uploadID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000038")
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000039")
	multipartRepo.upload = MultipartUpload{
		ID:              uploadID,
		UserID:          userID,
		ObjectKey:       "object-key",
		StorageUploadID: "storage-upload-id",
		EncryptedSize:   10,
		PartSize:        5,
		Status:          MultipartUploadStatusUploading,
	}

	// Act
	out, err := uc.UploadBinaryMultipartPart(context.Background(), UploadBinaryMultipartPartInput{
		UserID:     userID,
		UploadID:   uploadID,
		PartNumber: 2,
		PartSize:   5,
		Data:       bytes.NewReader([]byte("part2")),
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, uploadID, out.UploadID)
	assert.Equal(t, int32(2), out.Part.PartNumber)
	assert.Equal(t, int64(5), out.Part.Size)
	assert.Equal(t, "etag", out.Part.ETag)
	assert.Equal(t, "object-key", storage.putPartObjectKey)
	assert.Equal(t, "storage-upload-id", storage.putPartStorageID)
	assert.Equal(t, int32(2), storage.putPartNumber)
	assert.Equal(t, int64(5), storage.putPartSize)
	assert.Equal(t, []byte("part2"), storage.putPartData)
	require.Len(t, multipartRepo.upserted, 1)
	assert.Equal(t, uploadID, multipartRepo.upserted[0].UploadID)
	assert.Equal(t, int32(2), multipartRepo.upserted[0].PartNumber)
}

// TestRecordUseCase_UploadBinaryMultipartPart_FailWithInvalidInput проверяет ошибки валидации части до обращения к
// файловому хранилищу.
func TestRecordUseCase_UploadBinaryMultipartPart_FailWithInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		in   UploadBinaryMultipartPartInput
	}{
		{name: "nil data", in: UploadBinaryMultipartPartInput{Data: nil}},
		{
			name: "wrong part size",
			in:   UploadBinaryMultipartPartInput{PartNumber: 1, PartSize: 4, Data: bytes.NewReader([]byte("part"))},
		},
		{
			name: "wrong part number",
			in:   UploadBinaryMultipartPartInput{PartNumber: 3, PartSize: 5, Data: bytes.NewReader([]byte("part"))},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			uc, _, _, multipartRepo, storage, _, _ := newTestRecordUseCase(t)
			multipartRepo.upload = MultipartUpload{
				EncryptedSize: 10,
				PartSize:      5,
				Status:        MultipartUploadStatusUploading,
			}

			// Act
			_, err := uc.UploadBinaryMultipartPart(context.Background(), tt.in)

			// Assert
			require.ErrorIs(t, err, ErrMultipartUploadPartInvalid)
			assert.Empty(t, storage.putPartObjectKey)
			assert.Empty(t, multipartRepo.upserted)
		})
	}
}

// TestRecordUseCase_UploadBinaryMultipartPart_FailWithInactiveUpload проверяет запрет загрузки части в завершенную
// multipart-загрузку.
func TestRecordUseCase_UploadBinaryMultipartPart_FailWithInactiveUpload(t *testing.T) {
	// Arrange
	uc, _, _, multipartRepo, storage, _, _ := newTestRecordUseCase(t)
	multipartRepo.upload = MultipartUpload{Status: MultipartUploadStatusCompleted}

	// Act
	_, err := uc.UploadBinaryMultipartPart(context.Background(), UploadBinaryMultipartPartInput{
		PartNumber: 1,
		PartSize:   5,
		Data:       bytes.NewReader([]byte("part")),
	})

	// Assert
	require.ErrorIs(t, err, ErrMultipartUploadNotActive)
	assert.Empty(t, storage.putPartObjectKey)
}

// TestRecordUseCase_UploadBinaryMultipartPart_FailWithStorageError проверяет ошибку загрузки части в файловое
// хранилище.
func TestRecordUseCase_UploadBinaryMultipartPart_FailWithStorageError(t *testing.T) {
	// Arrange
	uc, _, _, multipartRepo, _, _, _ := newTestRecordUseCase(t)
	multipartRepo.upload = MultipartUpload{
		EncryptedSize: 5,
		PartSize:      5,
		Status:        MultipartUploadStatusUploading,
	}
	storage := uc.fileStorage.(*fileStorageStub)
	storage.putPartErr = errTest

	// Act
	_, err := uc.UploadBinaryMultipartPart(context.Background(), UploadBinaryMultipartPartInput{
		PartNumber: 1,
		PartSize:   5,
		Data:       bytes.NewReader([]byte("part")),
	})

	// Assert
	require.ErrorIs(t, err, errTest)
	assert.Empty(t, multipartRepo.upserted)
}

// TestRecordUseCase_StartBinaryMultipartUpload_ReplaceExistingBinary проверяет подготовку multipart-замены файла под
// guard-блокировкой пользователя.
func TestRecordUseCase_StartBinaryMultipartUpload_ReplaceExistingBinary(t *testing.T) {
	// Arrange
	uc, recordRepo, recordFileRepo, multipartRepo, storage, _, guard := newTestRecordUseCase(t)
	storage.objectKey = "users/user/records/record/files/new-file/payload"
	storage.storageUploadID = "storage-upload-id-3"
	recordRepo.version = 3
	recordID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000003")
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000004")
	oldFileCreatedAt := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	recordRepo.record = model.Record{
		ID:     recordID,
		UserID: userID,
		Type:   model.RecordTypeBinary,
		File: &model.RecordFile{
			ID:           uuid.MustParse("018f6b7c-0000-7000-8000-100000000005"),
			RecordID:     recordID,
			ObjectKey:    "old-object-key",
			UploadStatus: model.UploadStatusUploaded,
			CreatedAt:    oldFileCreatedAt,
		},
	}

	// Act
	out, err := uc.StartBinaryMultipartUpload(context.Background(), StartBinaryMultipartUploadInput{
		RecordID:         recordID,
		UserID:           userID,
		Title:            "new binary",
		Description:      "new description",
		EncryptedDEK:     []byte("new-encrypted-dek"),
		EncryptedPayload: []byte("new-encrypted-payload"),
		EncryptedSize:    10 * 1024 * 1024,
		PartSize:         5 * 1024 * 1024,
		ExpectedVersion:  2,
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, recordID, out.RecordID)
	assert.Equal(t, int64(3), out.Version)
	assert.Equal(t, model.UploadStatusUploading, out.UploadStatus)
	assert.Equal(t, 1, guard.calls)
	assert.Equal(t, []uuid.UUID{userID}, guard.lockedUserIDs)
	assert.True(t, guard.lockedInTx[0])
	require.Len(t, recordRepo.updated, 1)
	assert.Equal(t, recordID, recordRepo.updated[0].ID)
	assert.Equal(t, userID, recordRepo.updated[0].UserID)
	assert.Equal(t, "new binary", recordRepo.updated[0].Title)
	assert.Equal(t, []byte("new-encrypted-dek"), recordRepo.updated[0].EncryptedDEK.Data)
	assert.Equal(t, []byte("new-encrypted-payload"), recordRepo.updated[0].EncryptedPayload.Data)
	require.Len(t, recordFileRepo.replaced, 1)
	assert.Equal(t, recordRepo.record.File.ID, recordFileRepo.replaced[0].ID)
	assert.Equal(t, recordRepo.record.File.ID, storage.objectKeyFileID)
	assert.Equal(t, recordID, recordFileRepo.replaced[0].RecordID)
	assert.Equal(t, "users/user/records/record/files/new-file/payload", recordFileRepo.replaced[0].ObjectKey)
	assert.Equal(t, model.UploadStatusUploading, recordFileRepo.replaced[0].UploadStatus)
	require.Len(t, multipartRepo.created, 1)
	assert.Equal(t, out.UploadID, multipartRepo.created[0].ID)
	assert.Equal(t, int64(3), multipartRepo.created[0].RecordVersion)
	assert.Equal(t, "storage-upload-id-3", multipartRepo.created[0].StorageUploadID)
}

// TestRecordUseCase_StartBinaryMultipartUpload_ReplaceExistingBinaryFailWithInvalidRecordState проверяет, что ошибки
// состояния заменяемой бинарной записи возвращаются до открытия guard-блокировки.
func TestRecordUseCase_StartBinaryMultipartUpload_ReplaceExistingBinaryFailWithInvalidRecordState(t *testing.T) {
	tests := []struct {
		name   string
		record model.Record
		err    error
	}{
		{name: "not binary", record: model.Record{Type: model.RecordTypeText}, err: ErrRecordIsNotBinary},
		{name: "file is nil", record: model.Record{Type: model.RecordTypeBinary}, err: ErrRecordFileIsNotUploaded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			uc, recordRepo, recordFileRepo, _, _, _, guard := newTestRecordUseCase(t)
			recordRepo.record = tt.record

			// Act
			_, err := uc.StartBinaryMultipartUpload(context.Background(), StartBinaryMultipartUploadInput{
				RecordID:      uuid.MustParse("018f6b7c-0000-7000-8000-100000000013"),
				EncryptedSize: 1,
				PartSize:      1,
			})

			// Assert
			require.ErrorIs(t, err, tt.err)
			assert.Zero(t, guard.calls)
			assert.Empty(t, recordFileRepo.replaced)
		})
	}
}

// TestRecordUseCase_ListRecords проверяет успешное получение списка приватных записей.
func TestRecordUseCase_ListRecords(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _, _ := newTestRecordUseCase(t)
	items := []model.RecordListItem{{ID: uuid.MustParse("018f6b7c-0000-7000-8000-100000000003"), Title: "title"}}
	recordRepo.items = items

	// Act
	out, err := uc.ListRecords(context.Background(), ListRecordsInput{UserID: uuid.New()})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, items, out.Items)
}

// TestRecordUseCase_ListRecords_FailWithRepositoryError проверяет ошибку получения списка приватных записей.
func TestRecordUseCase_ListRecords_FailWithRepositoryError(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _, _ := newTestRecordUseCase(t)
	recordRepo.listErr = errTest

	// Act
	_, err := uc.ListRecords(context.Background(), ListRecordsInput{UserID: uuid.New()})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestRecordUseCase_GetRecord проверяет успешное получение приватной записи.
func TestRecordUseCase_GetRecord(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _, _ := newTestRecordUseCase(t)
	record := model.Record{ID: uuid.MustParse("018f6b7c-0000-7000-8000-100000000004"), Title: "title"}
	recordRepo.record = record

	// Act
	out, err := uc.GetRecord(context.Background(), GetRecordInput{RecordID: record.ID, UserID: uuid.New()})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, record, out.Record)
}

// TestRecordUseCase_GetRecord_FailWithNotFound проверяет ошибку отсутствующей приватной записи.
func TestRecordUseCase_GetRecord_FailWithNotFound(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _, _ := newTestRecordUseCase(t)
	recordRepo.getErr = ErrRecordNotFound

	// Act
	_, err := uc.GetRecord(context.Background(), GetRecordInput{})

	// Assert
	require.ErrorIs(t, err, ErrRecordNotFound)
}

// TestRecordUseCase_GetRecord_FailWithRepositoryError проверяет ошибку чтения приватной записи.
func TestRecordUseCase_GetRecord_FailWithRepositoryError(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _, _ := newTestRecordUseCase(t)
	recordRepo.getErr = errTest

	// Act
	_, err := uc.GetRecord(context.Background(), GetRecordInput{})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestRecordUseCase_UpdateRecord проверяет обновление приватной записи под guard-блокировкой пользователя.
func TestRecordUseCase_UpdateRecord(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _, guard := newTestRecordUseCase(t)
	recordRepo.version = 2
	recordID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000005")
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000006")

	// Act
	out, err := uc.UpdateRecord(context.Background(), UpdateRecordInput{
		RecordID: recordID, UserID: userID, Title: "new", Description: "desc", EncryptedDEK: []byte("dek"),
		EncryptedPayload: []byte("payload"), ExpectedVersion: 1,
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, recordID, out.RecordID)
	assert.Equal(t, int64(2), out.Version)
	require.Len(t, recordRepo.updated, 1)
	assert.Equal(t, recordID, recordRepo.updated[0].ID)
	assert.Equal(t, userID, recordRepo.updated[0].UserID)
	assert.Equal(t, []byte("dek"), recordRepo.updated[0].EncryptedDEK.Data)
	assert.Equal(t, []byte("payload"), recordRepo.updated[0].EncryptedPayload.Data)
	assert.Equal(t, 1, guard.calls)
	assert.Equal(t, []uuid.UUID{userID}, guard.lockedUserIDs)
	assert.True(t, guard.lockedInTx[0])
	assert.True(t, recordRepo.updatedInTx[0])
}

// TestRecordUseCase_UpdateRecord_FailWithKnownErrors проверяет ошибку отсутствующей приватной записи и ошибку
// конфликта версий.
func TestRecordUseCase_UpdateRecord_FailWithKnownErrors(t *testing.T) {
	tests := []error{ErrRecordNotFound, ErrRecordVersionConflict}
	for _, wantErr := range tests {
		t.Run(wantErr.Error(), func(t *testing.T) {
			// Arrange
			uc, recordRepo, _, _, _, _, _ := newTestRecordUseCase(t)
			recordRepo.updateErr = wantErr

			// Act
			_, err := uc.UpdateRecord(context.Background(), UpdateRecordInput{})

			// Assert
			require.ErrorIs(t, err, wantErr)
		})
	}
}

// TestRecordUseCase_UpdateRecord_FailWithRepositoryError проверяет неизвестную ошибку обновления приватной записи.
func TestRecordUseCase_UpdateRecord_FailWithRepositoryError(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _, _ := newTestRecordUseCase(t)
	recordRepo.updateErr = errTest

	// Act
	_, err := uc.UpdateRecord(context.Background(), UpdateRecordInput{})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestRecordUseCase_CompleteBinaryMultipartUpload проверяет завершение multipart-загрузки с проверкой контрольной
// суммы зашифрованного файла.
func TestRecordUseCase_CompleteBinaryMultipartUpload(t *testing.T) {
	// Arrange
	uc, _, recordFileRepo, multipartRepo, storage, tx, _ := newTestRecordUseCase(t)
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000031")
	uploadID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000032")
	recordID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000033")
	fileID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000034")
	encryptedFile := []byte("encrypted-file")
	encryptedSHA256 := testEncryptedSHA256(encryptedFile)
	multipartRepo.upload = MultipartUpload{
		ID:              uploadID,
		UserID:          userID,
		RecordID:        recordID,
		RecordVersion:   2,
		FileID:          fileID,
		ObjectKey:       "object-key",
		StorageUploadID: "storage-upload-id",
		EncryptedSize:   int64(len(encryptedFile)),
		PartSize:        int64(len(encryptedFile)),
		Status:          MultipartUploadStatusUploading,
	}
	multipartRepo.parts = []MultipartUploadPart{
		{UploadID: uploadID, PartNumber: 1, Size: int64(len(encryptedFile)), ETag: "etag"},
	}
	storage.getReader = io.NopCloser(bytes.NewReader(encryptedFile))

	// Act
	out, err := uc.CompleteBinaryMultipartUpload(context.Background(), CompleteBinaryMultipartUploadInput{
		UserID:          userID,
		UploadID:        uploadID,
		EncryptedSHA256: encryptedSHA256,
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, recordID, out.RecordID)
	assert.Equal(t, int64(2), out.Version)
	assert.Equal(t, model.UploadStatusUploaded, out.UploadStatus)
	assert.Equal(t, encryptedSHA256, out.EncryptedSHA256)
	assert.Equal(t, "object-key", storage.completeMultipartKey)
	assert.Equal(t, "storage-upload-id", storage.completeStorageID)
	assert.Equal(t, "object-key", storage.getObjectKey)
	assert.Equal(t, []MultipartUploadStatus{MultipartUploadStatusCompleted}, multipartRepo.statuses)
	require.Len(t, recordFileRepo.completed, 1)
	assert.Equal(t, fileID, recordFileRepo.completed[0].fileID)
	assert.Equal(t, encryptedSHA256, recordFileRepo.completed[0].encryptedSHA256)
	assert.Equal(t, 1, tx.calls)
	assert.True(t, multipartRepo.updatedInTx[0])
}

// TestRecordUseCase_CompleteBinaryMultipartUpload_FailWithChecksumMismatch проверяет, что собранный файл с другой
// контрольной суммой переводится в неуспешный статус.
func TestRecordUseCase_CompleteBinaryMultipartUpload_FailWithChecksumMismatch(t *testing.T) {
	// Arrange
	uc, _, recordFileRepo, multipartRepo, storage, tx, _ := newTestRecordUseCase(t)
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000035")
	uploadID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000036")
	fileID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000037")
	encryptedFile := []byte("encrypted-file")
	multipartRepo.upload = MultipartUpload{
		ID:              uploadID,
		UserID:          userID,
		FileID:          fileID,
		ObjectKey:       "object-key",
		StorageUploadID: "storage-upload-id",
		EncryptedSize:   int64(len(encryptedFile)),
		PartSize:        int64(len(encryptedFile)),
		Status:          MultipartUploadStatusUploading,
	}
	multipartRepo.parts = []MultipartUploadPart{
		{UploadID: uploadID, PartNumber: 1, Size: int64(len(encryptedFile)), ETag: "etag"},
	}
	storage.getReader = io.NopCloser(bytes.NewReader(encryptedFile))

	// Act
	_, err := uc.CompleteBinaryMultipartUpload(context.Background(), CompleteBinaryMultipartUploadInput{
		UserID:          userID,
		UploadID:        uploadID,
		EncryptedSHA256: strings.Repeat("0", 64),
	})

	// Assert
	require.ErrorIs(t, err, ErrMultipartUploadChecksumMismatch)
	assert.Equal(t, []MultipartUploadStatus{MultipartUploadStatusAborted}, multipartRepo.statuses)
	require.Len(t, recordFileRepo.updates, 1)
	assert.Equal(t, fileID, recordFileRepo.updates[0].fileID)
	assert.Equal(t, model.UploadStatusFailed, recordFileRepo.updates[0].status)
	assert.Equal(t, 1, tx.calls)
	assert.True(t, multipartRepo.updatedInTx[0])
}

// TestRecordUseCase_AbortBinaryMultipartUpload проверяет отмену активной multipart-загрузки в файловом хранилище и БД.
func TestRecordUseCase_AbortBinaryMultipartUpload(t *testing.T) {
	// Arrange
	uc, _, recordFileRepo, multipartRepo, storage, tx, _ := newTestRecordUseCase(t)
	uploadID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000040")
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000041")
	fileID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000042")
	multipartRepo.upload = MultipartUpload{
		ID:              uploadID,
		UserID:          userID,
		FileID:          fileID,
		ObjectKey:       "object-key",
		StorageUploadID: "storage-upload-id",
		Status:          MultipartUploadStatusUploading,
	}

	// Act
	out, err := uc.AbortBinaryMultipartUpload(context.Background(), AbortBinaryMultipartUploadInput{
		UserID:   userID,
		UploadID: uploadID,
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, uploadID, out.UploadID)
	assert.Equal(t, model.UploadStatusFailed, out.UploadStatus)
	assert.Equal(t, 1, storage.abortMultipartCallCnt)
	assert.Equal(t, "object-key", storage.abortMultipartKey)
	assert.Equal(t, "storage-upload-id", storage.abortStorageUploadID)
	assert.Equal(t, 1, tx.calls)
	assert.True(t, multipartRepo.updatedInTx[0])
	assert.Equal(t, []MultipartUploadStatus{MultipartUploadStatusAborted}, multipartRepo.statuses)
	require.Len(t, recordFileRepo.updates, 1)
	assert.Equal(t, fileID, recordFileRepo.updates[0].fileID)
	assert.Equal(t, model.UploadStatusFailed, recordFileRepo.updates[0].status)
}

// TestRecordUseCase_AbortBinaryMultipartUpload_Completed проверяет запрет отмены уже завершенной multipart-загрузки.
func TestRecordUseCase_AbortBinaryMultipartUpload_Completed(t *testing.T) {
	// Arrange
	uc, _, recordFileRepo, multipartRepo, storage, _, _ := newTestRecordUseCase(t)
	multipartRepo.upload = MultipartUpload{Status: MultipartUploadStatusCompleted}

	// Act
	_, err := uc.AbortBinaryMultipartUpload(context.Background(), AbortBinaryMultipartUploadInput{})

	// Assert
	require.ErrorIs(t, err, ErrMultipartUploadNotActive)
	assert.Zero(t, storage.abortMultipartCallCnt)
	assert.Empty(t, multipartRepo.statuses)
	assert.Empty(t, recordFileRepo.updates)
}

// TestRecordUseCase_AbortBinaryMultipartUpload_FailWithStorageError проверяет ошибку отмены multipart-загрузки в
// файловом хранилище.
func TestRecordUseCase_AbortBinaryMultipartUpload_FailWithStorageError(t *testing.T) {
	// Arrange
	uc, _, recordFileRepo, multipartRepo, storage, _, _ := newTestRecordUseCase(t)
	storage.abortMultipartErr = errTest
	multipartRepo.upload = MultipartUpload{
		ObjectKey:       "object-key",
		StorageUploadID: "storage-upload-id",
		Status:          MultipartUploadStatusUploading,
	}

	// Act
	_, err := uc.AbortBinaryMultipartUpload(context.Background(), AbortBinaryMultipartUploadInput{})

	// Assert
	require.ErrorIs(t, err, errTest)
	assert.Equal(t, 1, storage.abortMultipartCallCnt)
	assert.Empty(t, multipartRepo.statuses)
	assert.Empty(t, recordFileRepo.updates)
}

// TestRecordUseCase_DeleteRecord проверяет мягкое удаление приватной записи под guard-блокировкой пользователя.
func TestRecordUseCase_DeleteRecord(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _, guard := newTestRecordUseCase(t)
	recordID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000007")
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000008")

	// Act
	out, err := uc.DeleteRecord(context.Background(), DeleteRecordInput{RecordID: recordID, UserID: userID})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, recordID, out.RecordID)
	assert.Equal(t, []uuid.UUID{recordID}, recordRepo.deleted)
	assert.False(t, recordRepo.deletedAt.IsZero())
	assert.Equal(t, 1, guard.calls)
	assert.Equal(t, []uuid.UUID{userID}, guard.lockedUserIDs)
	assert.True(t, guard.lockedInTx[0])
	assert.True(t, recordRepo.deletedInTx[0])
}

// TestRecordUseCase_DeleteRecord_FailWithNotFound проверяет ошибку удаления отсутствующей приватной записи.
func TestRecordUseCase_DeleteRecord_FailWithNotFound(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _, _ := newTestRecordUseCase(t)
	recordRepo.deleteErr = ErrRecordNotFound

	// Act
	_, err := uc.DeleteRecord(context.Background(), DeleteRecordInput{})

	// Assert
	require.ErrorIs(t, err, ErrRecordNotFound)
}

// TestRecordUseCase_DeleteRecord_FailWithRepositoryError проверяет неизвестную ошибку удаления приватной записи.
func TestRecordUseCase_DeleteRecord_FailWithRepositoryError(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _, _ := newTestRecordUseCase(t)
	recordRepo.deleteErr = errTest

	// Act
	_, err := uc.DeleteRecord(context.Background(), DeleteRecordInput{})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestRecordUseCase_DownloadFile проверяет успешное скачивание файла приватной записи.
func TestRecordUseCase_DownloadFile(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, storage, _, _ := newTestRecordUseCase(t)
	fileData := []byte("encrypted-file")
	recordID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000008")
	recordRepo.record = model.Record{
		ID:   recordID,
		Type: model.RecordTypeBinary,
		File: &model.RecordFile{ObjectKey: "object-key", UploadStatus: model.UploadStatusUploaded},
	}
	storage.getReader = io.NopCloser(bytes.NewReader(fileData))

	// Act
	out, err := uc.DownloadFile(context.Background(), DownloadFileInput{RecordID: recordID, UserID: uuid.New()})

	// Assert
	require.NoError(t, err)
	defer func() {
		require.NoError(t, out.EncryptedFile.Close())
	}()
	got, err := io.ReadAll(out.EncryptedFile)
	require.NoError(t, err)
	assert.Equal(t, fileData, got)
	assert.Equal(t, "object-key", storage.getObjectKey)
}

// TestRecordUseCase_DownloadFile_FailWithGetRecordError проверяет ошибки чтения приватной записи перед скачиванием.
func TestRecordUseCase_DownloadFile_FailWithGetRecordError(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "not found", err: ErrRecordNotFound},
		{name: "repository error", err: errTest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			uc, recordRepo, _, _, _, _, _ := newTestRecordUseCase(t)
			recordRepo.getErr = tt.err

			// Act
			_, err := uc.DownloadFile(context.Background(), DownloadFileInput{})

			// Assert
			require.ErrorIs(t, err, tt.err)
		})
	}
}

// TestRecordUseCase_DownloadFile_FailWithInvalidRecordState проверяет ошибки состояния приватной записи.
func TestRecordUseCase_DownloadFile_FailWithInvalidRecordState(t *testing.T) {
	tests := []struct {
		name   string
		record model.Record
		err    error
	}{
		{name: "not binary", record: model.Record{Type: model.RecordTypeText}, err: ErrRecordIsNotBinary},
		{name: "file is nil", record: model.Record{Type: model.RecordTypeBinary}, err: ErrRecordFileIsNotUploaded},
		{
			name: "file is not uploaded",
			record: model.Record{
				Type: model.RecordTypeBinary,
				File: &model.RecordFile{UploadStatus: model.UploadStatusUploading},
			},
			err: ErrRecordFileIsNotUploaded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			uc, recordRepo, _, _, storage, _, _ := newTestRecordUseCase(t)
			recordRepo.record = tt.record

			// Act
			_, err := uc.DownloadFile(context.Background(), DownloadFileInput{})

			// Assert
			require.ErrorIs(t, err, tt.err)
			assert.Empty(t, storage.getObjectKey)
		})
	}
}

// TestRecordUseCase_DownloadFile_FailWithStorageError проверяет ошибку файлового хранилища.
func TestRecordUseCase_DownloadFile_FailWithStorageError(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, storage, _, _ := newTestRecordUseCase(t)
	recordRepo.record = model.Record{
		Type: model.RecordTypeBinary,
		File: &model.RecordFile{ObjectKey: "object-key", UploadStatus: model.UploadStatusUploaded},
	}
	storage.getErr = errTest

	// Act
	_, err := uc.DownloadFile(context.Background(), DownloadFileInput{})

	// Assert
	require.ErrorIs(t, err, errTest)
	assert.Equal(t, "object-key", storage.getObjectKey)
}
