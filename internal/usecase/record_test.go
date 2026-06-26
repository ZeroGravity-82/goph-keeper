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

type recordFileStatusUpdate struct {
	fileID    uuid.UUID
	status    model.UploadStatus
	updatedAt time.Time
}

type recordRepositoryStub struct {
	createErr   error
	getErr      error
	listErr     error
	updateErr   error
	deleteErr   error
	record      model.Record
	items       []model.RecordListItem
	version     int64
	created     []model.Record
	createdInTx []bool
	updated     []model.Record
	deleted     []uuid.UUID
	deletedAt   time.Time
}

func (r *recordRepositoryStub) Create(ctx context.Context, record model.Record) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = append(r.created, record)
	r.createdInTx = append(r.createdInTx, ctx.Value(txContextKey{}) == true)
	return nil
}

func (r *recordRepositoryStub) GetByIDAndUserID(context.Context, uuid.UUID, uuid.UUID) (model.Record, error) {
	if r.getErr != nil {
		return model.Record{}, r.getErr
	}
	return r.record, nil
}

func (r *recordRepositoryStub) ListByUserID(context.Context, uuid.UUID) ([]model.RecordListItem, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.items, nil
}

func (r *recordRepositoryStub) Update(_ context.Context, record model.Record, _ int64) (int64, error) {
	if r.updateErr != nil {
		return 0, r.updateErr
	}
	r.updated = append(r.updated, record)
	return r.version, nil
}

func (r *recordRepositoryStub) Delete(_ context.Context, recordID uuid.UUID, _ uuid.UUID, deletedAt time.Time) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deleted = append(r.deleted, recordID)
	r.deletedAt = deletedAt
	return nil
}

type recordFileRepositoryStub struct {
	createErr   error
	updateErr   error
	created     []model.RecordFile
	createdInTx []bool
	updates     []recordFileStatusUpdate
}

func (r *recordFileRepositoryStub) Create(ctx context.Context, file model.RecordFile) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = append(r.created, file)
	r.createdInTx = append(r.createdInTx, ctx.Value(txContextKey{}) == true)
	return nil
}

func (r *recordFileRepositoryStub) UpdateUploadStatus(
	_ context.Context,
	fileID uuid.UUID,
	status model.UploadStatus,
	updatedAt time.Time,
) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.updates = append(r.updates, recordFileStatusUpdate{fileID: fileID, status: status, updatedAt: updatedAt})
	return nil
}

type fileStorageStub struct {
	objectKey    string
	putErr       error
	putWritten   *int64
	putObjectKey string
	putSize      int64
	putData      []byte
	getErr       error
	getObjectKey string
	getReader    io.ReadCloser
}

func (s *fileStorageStub) ObjectKey(uuid.UUID, uuid.UUID, uuid.UUID) string {
	if s.objectKey == "" {
		return "object-key"
	}
	return s.objectKey
}

func (s *fileStorageStub) Put(_ context.Context, objectKey string, data io.Reader, size int64) (int64, error) {
	s.putObjectKey = objectKey
	s.putSize = size
	b, readErr := io.ReadAll(data)
	if readErr != nil {
		return 0, readErr
	}
	s.putData = b
	if s.putErr != nil {
		return 0, s.putErr
	}
	if s.putWritten != nil {
		return *s.putWritten, nil
	}
	return int64(len(b)), nil
}

func (s *fileStorageStub) Get(_ context.Context, objectKey string) (io.ReadCloser, error) {
	s.getObjectKey = objectKey
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.getReader != nil {
		return s.getReader, nil
	}
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func newTestRecordUseCase(t *testing.T) (
	*RecordUseCase,
	*recordRepositoryStub,
	*recordFileRepositoryStub,
	*fileStorageStub,
	*transactorStub,
) {
	t.Helper()

	recordRepo := &recordRepositoryStub{}
	recordFileRepo := &recordFileRepositoryStub{}
	storage := &fileStorageStub{}
	tx := &transactorStub{}
	uc, err := NewRecordUseCase(recordRepo, recordFileRepo, storage, tx)
	require.NoError(t, err)
	return uc, recordRepo, recordFileRepo, storage, tx
}

func ptrInt64(v int64) *int64 {
	return &v
}

// TestRecordUseCase_CreateRecord проверяет успешное создание обычной приватной записи.
func TestRecordUseCase_CreateRecord(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _ := newTestRecordUseCase(t)
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
	uc, recordRepo, _, _, _ := newTestRecordUseCase(t)

	// Act
	_, err := uc.CreateRecord(context.Background(), CreateRecordInput{Type: model.RecordTypeBinary})

	// Assert
	require.ErrorIs(t, err, ErrBinaryRecordNotSupported)
	assert.Empty(t, recordRepo.created)
}

// TestRecordUseCase_CreateRecord_FailWithRepositoryError проверяет ошибку сохранения приватной записи.
func TestRecordUseCase_CreateRecord_FailWithRepositoryError(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _ := newTestRecordUseCase(t)
	recordRepo.createErr = errTest

	// Act
	_, err := uc.CreateRecord(context.Background(), CreateRecordInput{Type: model.RecordTypeText})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestRecordUseCase_CreateBinaryRecord проверяет успешное создание бинарной приватной записи и загрузку файла.
func TestRecordUseCase_CreateBinaryRecord(t *testing.T) {
	// Arrange
	uc, recordRepo, recordFileRepo, storage, tx := newTestRecordUseCase(t)
	storage.objectKey = "users/user/records/record/files/file/payload"
	fileData := []byte("file")
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-100000000002")

	// Act
	out, err := uc.CreateBinaryRecord(context.Background(), CreateBinaryRecordInput{
		UserID:           userID,
		Title:            "binary",
		Description:      "description",
		EncryptedDEK:     []byte("encrypted-dek"),
		EncryptedPayload: []byte("encrypted-payload"),
		EncryptedFile:    bytes.NewReader(fileData),
		EncryptedSize:    int64(len(fileData)),
		UploadMode:       model.UploadModeSinglePart,
	})

	// Assert
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, out.RecordID)
	assert.Equal(t, int64(1), out.Version)
	assert.Equal(t, model.UploadStatusUploaded, out.UploadStatus)
	assert.Equal(t, 1, tx.calls)
	require.Len(t, recordRepo.created, 1)
	require.Len(t, recordFileRepo.created, 1)
	assert.True(t, recordRepo.createdInTx[0])
	assert.True(t, recordFileRepo.createdInTx[0])
	assert.Equal(t, model.RecordTypeBinary, recordRepo.created[0].Type)
	assert.Equal(t, "users/user/records/record/files/file/payload", recordFileRepo.created[0].ObjectKey)
	assert.Equal(t, model.UploadStatusUploading, recordFileRepo.created[0].UploadStatus)
	assert.Equal(t, fileData, storage.putData)
	assert.Equal(t, int64(len(fileData)), storage.putSize)
	require.Len(t, recordFileRepo.updates, 1)
	assert.Equal(t, model.UploadStatusUploaded, recordFileRepo.updates[0].status)
}

// TestRecordUseCase_CreateBinaryRecord_FailWithInvalidInput проверяет ошибки валидации входных данных.
func TestRecordUseCase_CreateBinaryRecord_FailWithInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		in   CreateBinaryRecordInput
		err  error
	}{
		{
			name: "unsupported upload mode",
			in:   CreateBinaryRecordInput{EncryptedSize: 1, UploadMode: model.UploadModeMultiPart},
			err:  ErrUploadModeNotSupported,
		},
		{
			name: "zero encrypted size",
			in:   CreateBinaryRecordInput{EncryptedSize: 0, UploadMode: model.UploadModeSinglePart},
			err:  ErrInvalidBinaryEncryptedSize,
		},
		{
			name: "too large encrypted size",
			in: CreateBinaryRecordInput{
				EncryptedSize: int64(MaxEncryptedFileSize()) + 1,
				UploadMode:    model.UploadModeSinglePart,
			},
			err: ErrInvalidBinaryEncryptedSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			uc, recordRepo, recordFileRepo, storage, tx := newTestRecordUseCase(t)

			// Act
			_, err := uc.CreateBinaryRecord(context.Background(), tt.in)

			// Assert
			require.ErrorIs(t, err, tt.err)
			assert.Zero(t, tx.calls)
			assert.Empty(t, recordRepo.created)
			assert.Empty(t, recordFileRepo.created)
			assert.Empty(t, storage.putData)
		})
	}
}

// TestRecordUseCase_CreateBinaryRecord_FailWithCreateRecordError проверяет ошибку сохранения record.
func TestRecordUseCase_CreateBinaryRecord_FailWithCreateRecordError(t *testing.T) {
	// Arrange
	uc, recordRepo, recordFileRepo, storage, tx := newTestRecordUseCase(t)
	recordRepo.createErr = errTest

	// Act
	_, err := uc.CreateBinaryRecord(context.Background(), CreateBinaryRecordInput{
		EncryptedFile: bytes.NewReader([]byte("file")), EncryptedSize: 4, UploadMode: model.UploadModeSinglePart,
	})

	// Assert
	require.ErrorIs(t, err, errTest)
	assert.Equal(t, 1, tx.calls)
	assert.Empty(t, recordFileRepo.created)
	assert.Empty(t, storage.putData)
}

// TestRecordUseCase_CreateBinaryRecord_FailWithCreateRecordFileError проверяет ошибку сохранения record_file.
func TestRecordUseCase_CreateBinaryRecord_FailWithCreateRecordFileError(t *testing.T) {
	// Arrange
	uc, recordRepo, recordFileRepo, storage, tx := newTestRecordUseCase(t)
	recordFileRepo.createErr = errTest

	// Act
	_, err := uc.CreateBinaryRecord(context.Background(), CreateBinaryRecordInput{
		EncryptedFile: bytes.NewReader([]byte("file")), EncryptedSize: 4, UploadMode: model.UploadModeSinglePart,
	})

	// Assert
	require.ErrorIs(t, err, errTest)
	assert.Equal(t, 1, tx.calls)
	require.Len(t, recordRepo.created, 1)
	assert.Empty(t, storage.putData)
}

// TestRecordUseCase_CreateBinaryRecord_FailWithStorageError проверяет ошибку загрузки файла в хранилище.
func TestRecordUseCase_CreateBinaryRecord_FailWithStorageError(t *testing.T) {
	// Arrange
	uc, _, recordFileRepo, storage, _ := newTestRecordUseCase(t)
	storage.putErr = errTest

	// Act
	_, err := uc.CreateBinaryRecord(context.Background(), CreateBinaryRecordInput{
		EncryptedFile: bytes.NewReader([]byte("file")), EncryptedSize: 4, UploadMode: model.UploadModeSinglePart,
	})

	// Assert
	require.ErrorIs(t, err, errTest)
	require.Len(t, recordFileRepo.updates, 1)
	assert.Equal(t, model.UploadStatusFailed, recordFileRepo.updates[0].status)
}

// TestRecordUseCase_CreateBinaryRecord_FailWithStorageSizeMismatch проверяет ошибку несовпадения размера из хранилища.
func TestRecordUseCase_CreateBinaryRecord_FailWithStorageSizeMismatch(t *testing.T) {
	// Arrange
	uc, _, recordFileRepo, storage, _ := newTestRecordUseCase(t)
	storage.putErr = ErrBinaryEncryptedSizeMismatch

	// Act
	_, err := uc.CreateBinaryRecord(context.Background(), CreateBinaryRecordInput{
		EncryptedFile: bytes.NewReader([]byte("file")), EncryptedSize: 4, UploadMode: model.UploadModeSinglePart,
	})

	// Assert
	require.ErrorIs(t, err, ErrBinaryEncryptedSizeMismatch)
	require.Len(t, recordFileRepo.updates, 1)
	assert.Equal(t, model.UploadStatusFailed, recordFileRepo.updates[0].status)
}

// TestRecordUseCase_CreateBinaryRecord_FailWithWrittenSizeMismatch проверяет несовпадение ожидаемого и фактического
// размера.
func TestRecordUseCase_CreateBinaryRecord_FailWithWrittenSizeMismatch(t *testing.T) {
	// Arrange
	uc, _, recordFileRepo, storage, _ := newTestRecordUseCase(t)
	storage.putWritten = ptrInt64(3)

	// Act
	_, err := uc.CreateBinaryRecord(context.Background(), CreateBinaryRecordInput{
		EncryptedFile: bytes.NewReader([]byte("file")), EncryptedSize: 4, UploadMode: model.UploadModeSinglePart,
	})

	// Assert
	require.ErrorIs(t, err, ErrBinaryEncryptedSizeMismatch)
	require.Len(t, recordFileRepo.updates, 1)
	assert.Equal(t, model.UploadStatusFailed, recordFileRepo.updates[0].status)
}

// TestRecordUseCase_CreateBinaryRecord_FailWithUpdateUploadedStatusError проверяет ошибку фиксации успешной загрузки.
func TestRecordUseCase_CreateBinaryRecord_FailWithUpdateUploadedStatusError(t *testing.T) {
	// Arrange
	uc, _, recordFileRepo, _, _ := newTestRecordUseCase(t)
	recordFileRepo.updateErr = errTest

	// Act
	_, err := uc.CreateBinaryRecord(context.Background(), CreateBinaryRecordInput{
		EncryptedFile: bytes.NewReader([]byte("file")), EncryptedSize: 4, UploadMode: model.UploadModeSinglePart,
	})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestRecordUseCase_ListRecords проверяет успешное получение списка приватных записей.
func TestRecordUseCase_ListRecords(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _ := newTestRecordUseCase(t)
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
	uc, recordRepo, _, _, _ := newTestRecordUseCase(t)
	recordRepo.listErr = errTest

	// Act
	_, err := uc.ListRecords(context.Background(), ListRecordsInput{UserID: uuid.New()})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestRecordUseCase_GetRecord проверяет успешное получение приватной записи.
func TestRecordUseCase_GetRecord(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _ := newTestRecordUseCase(t)
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
	uc, recordRepo, _, _, _ := newTestRecordUseCase(t)
	recordRepo.getErr = ErrRecordNotFound

	// Act
	_, err := uc.GetRecord(context.Background(), GetRecordInput{})

	// Assert
	require.ErrorIs(t, err, ErrRecordNotFound)
}

// TestRecordUseCase_GetRecord_FailWithRepositoryError проверяет ошибку чтения приватной записи.
func TestRecordUseCase_GetRecord_FailWithRepositoryError(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _ := newTestRecordUseCase(t)
	recordRepo.getErr = errTest

	// Act
	_, err := uc.GetRecord(context.Background(), GetRecordInput{})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestRecordUseCase_UpdateRecord проверяет успешное обновление приватной записи.
func TestRecordUseCase_UpdateRecord(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _ := newTestRecordUseCase(t)
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
			uc, recordRepo, _, _, _ := newTestRecordUseCase(t)
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
	uc, recordRepo, _, _, _ := newTestRecordUseCase(t)
	recordRepo.updateErr = errTest

	// Act
	_, err := uc.UpdateRecord(context.Background(), UpdateRecordInput{})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestRecordUseCase_DeleteRecord проверяет мягкое удаление приватной записи.
func TestRecordUseCase_DeleteRecord(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _ := newTestRecordUseCase(t)
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
	uc, recordRepo, _, _, _ := newTestRecordUseCase(t)
	recordRepo.deleteErr = ErrRecordNotFound

	// Act
	_, err := uc.DeleteRecord(context.Background(), DeleteRecordInput{})

	// Assert
	require.ErrorIs(t, err, ErrRecordNotFound)
}

// TestRecordUseCase_DeleteRecord_FailWithRepositoryError проверяет неизвестную ошибку удаления приватной записи.
func TestRecordUseCase_DeleteRecord_FailWithRepositoryError(t *testing.T) {
	// Arrange
	uc, recordRepo, _, _, _ := newTestRecordUseCase(t)
	recordRepo.deleteErr = errTest

	// Act
	_, err := uc.DeleteRecord(context.Background(), DeleteRecordInput{})

	// Assert
	require.ErrorIs(t, err, errTest)
}

// TestRecordUseCase_DownloadFile проверяет успешное скачивание файла приватной записи.
func TestRecordUseCase_DownloadFile(t *testing.T) {
	// Arrange
	uc, recordRepo, _, storage, _ := newTestRecordUseCase(t)
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
			uc, recordRepo, _, _, _ := newTestRecordUseCase(t)
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
			uc, recordRepo, _, storage, _ := newTestRecordUseCase(t)
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
	uc, recordRepo, _, storage, _ := newTestRecordUseCase(t)
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
