package usecase

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"zerogravity-82/goph-keeper/internal/domain/model"
)

type recordFileStatusUpdate struct {
	fileID    uuid.UUID
	status    model.UploadStatus
	updatedAt time.Time
}

type recordFileCompleteUpdate struct {
	fileID          uuid.UUID
	encryptedSHA256 string
	updatedAt       time.Time
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
	updatedInTx []bool
	deleted     []uuid.UUID
	deletedAt   time.Time
	deletedInTx []bool
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

func (r *recordRepositoryStub) Update(ctx context.Context, record model.Record, _ int64) (int64, error) {
	if r.updateErr != nil {
		return 0, r.updateErr
	}
	r.updated = append(r.updated, record)
	r.updatedInTx = append(r.updatedInTx, ctx.Value(txContextKey{}) == true)
	return r.version, nil
}

func (r *recordRepositoryStub) Delete(ctx context.Context, recordID uuid.UUID, _ uuid.UUID, deletedAt time.Time) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deleted = append(r.deleted, recordID)
	r.deletedAt = deletedAt
	r.deletedInTx = append(r.deletedInTx, ctx.Value(txContextKey{}) == true)
	return nil
}

type recordFileRepositoryStub struct {
	createErr   error
	replaceErr  error
	updateErr   error
	completeErr error
	created     []model.RecordFile
	replaced    []model.RecordFile
	createdInTx []bool
	updates     []recordFileStatusUpdate
	completed   []recordFileCompleteUpdate
}

func (r *recordFileRepositoryStub) Create(ctx context.Context, file model.RecordFile) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = append(r.created, file)
	r.createdInTx = append(r.createdInTx, ctx.Value(txContextKey{}) == true)
	return nil
}

func (r *recordFileRepositoryStub) Replace(_ context.Context, file model.RecordFile) error {
	if r.replaceErr != nil {
		return r.replaceErr
	}
	r.replaced = append(r.replaced, file)
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

func (r *recordFileRepositoryStub) CompleteUpload(
	_ context.Context,
	fileID uuid.UUID,
	encryptedSHA256 string,
	updatedAt time.Time,
) error {
	if r.completeErr != nil {
		return r.completeErr
	}
	r.completed = append(r.completed, recordFileCompleteUpdate{
		fileID:          fileID,
		encryptedSHA256: encryptedSHA256,
		updatedAt:       updatedAt,
	})
	return nil
}

type multipartUploadRepositoryStub struct {
	createErr   error
	getErr      error
	upsertErr   error
	listErr     error
	updateErr   error
	upload      MultipartUpload
	parts       []MultipartUploadPart
	created     []MultipartUpload
	upserted    []MultipartUploadPart
	statuses    []MultipartUploadStatus
	updatedInTx []bool
}

func (r *multipartUploadRepositoryStub) Create(_ context.Context, upload MultipartUpload) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = append(r.created, upload)
	return nil
}

func (r *multipartUploadRepositoryStub) GetByIDAndUserID(
	context.Context,
	uuid.UUID,
	uuid.UUID,
) (MultipartUpload, error) {
	if r.getErr != nil {
		return MultipartUpload{}, r.getErr
	}
	return r.upload, nil
}

func (r *multipartUploadRepositoryStub) UpsertPart(_ context.Context, part MultipartUploadPart) error {
	if r.upsertErr != nil {
		return r.upsertErr
	}
	r.upserted = append(r.upserted, part)
	return nil
}

func (r *multipartUploadRepositoryStub) ListParts(context.Context, uuid.UUID) ([]MultipartUploadPart, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.parts, nil
}

func (r *multipartUploadRepositoryStub) UpdateStatus(
	ctx context.Context,
	_ uuid.UUID,
	status MultipartUploadStatus,
	_ time.Time,
	_ *time.Time,
) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.statuses = append(r.statuses, status)
	r.updatedInTx = append(r.updatedInTx, ctx.Value(txContextKey{}) == true)
	return nil
}

type fileStorageStub struct {
	objectKey             string
	createMultipartErr    error
	createMultipartKey    string
	storageUploadID       string
	putPartErr            error
	putPartObjectKey      string
	putPartStorageID      string
	putPartNumber         int32
	putPartSize           int64
	putPartData           []byte
	completeMultipartErr  error
	completeMultipartKey  string
	completeStorageID     string
	completeParts         []MultipartUploadPart
	abortMultipartErr     error
	abortMultipartCallCnt int
	abortMultipartKey     string
	abortStorageUploadID  string
	objectKeyFileID       uuid.UUID
	getErr                error
	getObjectKey          string
	getReader             io.ReadCloser
}

func (s *fileStorageStub) ObjectKey(_, _, fileID uuid.UUID) string {
	s.objectKeyFileID = fileID
	if s.objectKey == "" {
		return "object-key"
	}
	return s.objectKey
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

func (s *fileStorageStub) CreateMultipartUpload(_ context.Context, objectKey string) (string, error) {
	s.createMultipartKey = objectKey
	if s.createMultipartErr != nil {
		return "", s.createMultipartErr
	}
	if s.storageUploadID == "" {
		return "storage-upload-id", nil
	}
	return s.storageUploadID, nil
}

func (s *fileStorageStub) PutMultipartPart(
	_ context.Context,
	objectKey string,
	uploadID string,
	partNumber int32,
	data io.Reader,
	size int64,
) (MultipartUploadPart, error) {
	s.putPartObjectKey = objectKey
	s.putPartStorageID = uploadID
	s.putPartNumber = partNumber
	s.putPartSize = size
	b, readErr := io.ReadAll(data)
	if readErr != nil {
		return MultipartUploadPart{}, readErr
	}
	s.putPartData = b
	if s.putPartErr != nil {
		return MultipartUploadPart{}, s.putPartErr
	}
	return MultipartUploadPart{PartNumber: partNumber, Size: size, ETag: "etag"}, nil
}

func (s *fileStorageStub) CompleteMultipartUpload(
	_ context.Context,
	objectKey string,
	uploadID string,
	parts []MultipartUploadPart,
) error {
	s.completeMultipartKey = objectKey
	s.completeStorageID = uploadID
	s.completeParts = append([]MultipartUploadPart(nil), parts...)
	return s.completeMultipartErr
}

func (s *fileStorageStub) AbortMultipartUpload(_ context.Context, objectKey string, uploadID string) error {
	s.abortMultipartCallCnt++
	s.abortMultipartKey = objectKey
	s.abortStorageUploadID = uploadID
	return s.abortMultipartErr
}

func newTestRecordUseCase(t *testing.T) (
	*RecordUseCase,
	*recordRepositoryStub,
	*recordFileRepositoryStub,
	*multipartUploadRepositoryStub,
	*fileStorageStub,
	*transactorStub,
	*recordMutationGuardStub,
) {
	t.Helper()

	recordRepo := &recordRepositoryStub{}
	recordFileRepo := &recordFileRepositoryStub{}
	multipartRepo := &multipartUploadRepositoryStub{}
	storage := &fileStorageStub{}
	tx := &transactorStub{}
	guard := &recordMutationGuardStub{}
	uc, err := NewRecordUseCase(recordRepo, recordFileRepo, multipartRepo, storage, tx, guard)
	require.NoError(t, err)
	return uc, recordRepo, recordFileRepo, multipartRepo, storage, tx, guard
}
