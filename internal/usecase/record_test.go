package usecase

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/goph-keeper/internal/domain/model"
)

// TestRecordUseCase_CreateRecord проверяет успешное создание обычной приватной записи.
func TestRecordUseCase_CreateRecord(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _ := newTestRecordUseCase(t)
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
}

// TestRecordUseCase_CreateRecord_FailWithBinaryType проверяет запрет создания бинарной записи обычным сценарием.
func TestRecordUseCase_CreateRecord_FailWithBinaryType(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _ := newTestRecordUseCase(t)

	// Act
	_, err := uc.CreateRecord(context.Background(), CreateRecordInput{Type: model.RecordTypeBinary})

	// Assert
	require.ErrorIs(t, err, ErrBinaryRecordNotSupported)
	assert.Empty(t, recordRepo.created)
}

// TestRecordUseCase_CreateRecord_FailWithRepositoryError проверяет ошибку сохранения приватной записи.
func TestRecordUseCase_CreateRecord_FailWithRepositoryError(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _ := newTestRecordUseCase(t)
	recordRepo.createErr = errTest

	// Act
	_, err := uc.CreateRecord(context.Background(), CreateRecordInput{Type: model.RecordTypeText})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestRecordUseCase_StartBinaryMultipartUpload проверяет создание записи, файла и серверной сессии multipart-загрузки.
func TestRecordUseCase_StartBinaryMultipartUpload(t *testing.T) {
	// Arrange
	uc, recordRepo, recordFileRepo, multipartRepo, storage, tx := newTestRecordUseCase(t)
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
	assert.Equal(t, 1, tx.calls)
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
	uc, _, _, multipartRepo, storage, _ := newTestRecordUseCase(t)
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
	uc, _, _, multipartRepo, _, _ := newTestRecordUseCase(t)
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

// TestRecordUseCase_StartBinaryMultipartUpload_ReplaceExistingBinary проверяет подготовку multipart-замены файла.
func TestRecordUseCase_StartBinaryMultipartUpload_ReplaceExistingBinary(t *testing.T) {
	// Arrange
	uc, recordRepo, recordFileRepo, multipartRepo, storage, tx := newTestRecordUseCase(t)
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
	assert.Equal(t, 1, tx.calls)
	require.Len(t, recordRepo.updated, 1)
	assert.Equal(t, recordID, recordRepo.updated[0].ID)
	assert.Equal(t, userID, recordRepo.updated[0].UserID)
	assert.Equal(t, "new binary", recordRepo.updated[0].Title)
	assert.Equal(t, []byte("new-encrypted-dek"), recordRepo.updated[0].EncryptedDEK.Data)
	assert.Equal(t, []byte("new-encrypted-payload"), recordRepo.updated[0].EncryptedPayload.Data)
	require.Len(t, recordFileRepo.replaced, 1)
	assert.NotEqual(t, recordRepo.record.File.ID, recordFileRepo.replaced[0].ID)
	assert.Equal(t, recordID, recordFileRepo.replaced[0].RecordID)
	assert.Equal(t, "users/user/records/record/files/new-file/payload", recordFileRepo.replaced[0].ObjectKey)
	assert.Equal(t, model.UploadStatusUploading, recordFileRepo.replaced[0].UploadStatus)
	require.Len(t, multipartRepo.created, 1)
	assert.Equal(t, out.UploadID, multipartRepo.created[0].ID)
	assert.Equal(t, int64(3), multipartRepo.created[0].RecordVersion)
	assert.Equal(t, "storage-upload-id-3", multipartRepo.created[0].StorageUploadID)
}

// TestRecordUseCase_StartBinaryMultipartUpload_ReplaceExistingBinaryFailWithInvalidRecordState проверяет ошибки
// состояния заменяемой бинарной приватной записи.
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
			uc, recordRepo, recordFileRepo, _, _, tx := newTestRecordUseCase(t)
			recordRepo.record = tt.record

			// Act
			_, err := uc.StartBinaryMultipartUpload(context.Background(), StartBinaryMultipartUploadInput{
				RecordID:      uuid.MustParse("018f6b7c-0000-7000-8000-100000000013"),
				EncryptedSize: 1,
				PartSize:      1,
			})

			// Assert
			require.ErrorIs(t, err, tt.err)
			assert.Zero(t, tx.calls)
			assert.Empty(t, recordFileRepo.replaced)
		})
	}
}

// TestRecordUseCase_ListRecords проверяет успешное получение списка приватных записей.
func TestRecordUseCase_ListRecords(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _ := newTestRecordUseCase(t)
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
	uc, recordRepo, _, _, _, _ := newTestRecordUseCase(t)
	recordRepo.listErr = errTest

	// Act
	_, err := uc.ListRecords(context.Background(), ListRecordsInput{UserID: uuid.New()})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestRecordUseCase_GetRecord проверяет успешное получение приватной записи.
func TestRecordUseCase_GetRecord(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _ := newTestRecordUseCase(t)
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
	uc, recordRepo, _, _, _, _ := newTestRecordUseCase(t)
	recordRepo.getErr = ErrRecordNotFound

	// Act
	_, err := uc.GetRecord(context.Background(), GetRecordInput{})

	// Assert
	require.ErrorIs(t, err, ErrRecordNotFound)
}

// TestRecordUseCase_GetRecord_FailWithRepositoryError проверяет ошибку чтения приватной записи.
func TestRecordUseCase_GetRecord_FailWithRepositoryError(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _ := newTestRecordUseCase(t)
	recordRepo.getErr = errTest

	// Act
	_, err := uc.GetRecord(context.Background(), GetRecordInput{})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestRecordUseCase_UpdateRecord проверяет успешное обновление приватной записи.
func TestRecordUseCase_UpdateRecord(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _ := newTestRecordUseCase(t)
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
}

// TestRecordUseCase_UpdateRecord_FailWithKnownErrors проверяет ошибку отсутствующей приватной записи и ошибку
// конфликта версий.
func TestRecordUseCase_UpdateRecord_FailWithKnownErrors(t *testing.T) {
	tests := []error{ErrRecordNotFound, ErrRecordVersionConflict}
	for _, wantErr := range tests {
		t.Run(wantErr.Error(), func(t *testing.T) {
			// Arrange
			uc, recordRepo, _, _, _, _ := newTestRecordUseCase(t)
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
	uc, recordRepo, _, _, _, _ := newTestRecordUseCase(t)
	recordRepo.updateErr = errTest

	// Act
	_, err := uc.UpdateRecord(context.Background(), UpdateRecordInput{})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestRecordUseCase_DeleteRecord проверяет мягкое удаление приватной записи.
func TestRecordUseCase_DeleteRecord(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _ := newTestRecordUseCase(t)
	recordID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000007")

	// Act
	out, err := uc.DeleteRecord(context.Background(), DeleteRecordInput{RecordID: recordID, UserID: uuid.New()})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, recordID, out.RecordID)
	assert.Equal(t, []uuid.UUID{recordID}, recordRepo.deleted)
	assert.False(t, recordRepo.deletedAt.IsZero())
}

// TestRecordUseCase_DeleteRecord_FailWithNotFound проверяет ошибку удаления отсутствующей приватной записи.
func TestRecordUseCase_DeleteRecord_FailWithNotFound(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _ := newTestRecordUseCase(t)
	recordRepo.deleteErr = ErrRecordNotFound

	// Act
	_, err := uc.DeleteRecord(context.Background(), DeleteRecordInput{})

	// Assert
	require.ErrorIs(t, err, ErrRecordNotFound)
}

// TestRecordUseCase_DeleteRecord_FailWithRepositoryError проверяет неизвестную ошибку удаления приватной записи.
func TestRecordUseCase_DeleteRecord_FailWithRepositoryError(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _, _ := newTestRecordUseCase(t)
	recordRepo.deleteErr = errTest

	// Act
	_, err := uc.DeleteRecord(context.Background(), DeleteRecordInput{})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestRecordUseCase_DownloadFile проверяет успешное скачивание файла приватной записи.
func TestRecordUseCase_DownloadFile(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, storage, _ := newTestRecordUseCase(t)
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
	defer out.EncryptedFile.Close()
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
			uc, recordRepo, _, _, _, _ := newTestRecordUseCase(t)
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
			uc, recordRepo, _, _, storage, _ := newTestRecordUseCase(t)
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
	uc, recordRepo, _, _, storage, _ := newTestRecordUseCase(t)
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
