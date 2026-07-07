package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"zerogravity-82/goph-keeper/internal/domain/model"
)

const initialRecordVersion int64 = 1

// CreateRecordInput описывает входные данные сценария создания приватной записи.
type CreateRecordInput struct {
	UserID           uuid.UUID
	Type             model.RecordType
	Title            string
	Description      string
	EncryptedDEK     []byte
	EncryptedPayload []byte
}

// CreateRecordOutput описывает результат создания приватной записи.
type CreateRecordOutput struct {
	RecordID uuid.UUID
	Version  int64
}

// CreateBinaryRecordInput описывает входные данные сценария создания бинарной приватной записи.
type CreateBinaryRecordInput struct {
	UserID           uuid.UUID
	Title            string
	Description      string
	EncryptedDEK     []byte
	EncryptedPayload []byte
	EncryptedFile    io.Reader
	EncryptedSize    int64
	UploadMode       model.UploadMode
}

// CreateBinaryRecordOutput описывает результат создания бинарной приватной записи.
type CreateBinaryRecordOutput struct {
	RecordID     uuid.UUID
	Version      int64
	UploadStatus model.UploadStatus
}

// UpdateBinaryRecordInput описывает входные данные сценария обновления бинарной приватной записи.
type UpdateBinaryRecordInput struct {
	RecordID         uuid.UUID
	UserID           uuid.UUID
	Title            string
	Description      string
	EncryptedDEK     []byte
	EncryptedPayload []byte
	EncryptedFile    io.Reader
	EncryptedSize    int64
	UploadMode       model.UploadMode
	ExpectedVersion  int64
}

// UpdateBinaryRecordOutput описывает результат обновления бинарной приватной записи.
type UpdateBinaryRecordOutput struct {
	RecordID     uuid.UUID
	Version      int64
	UploadStatus model.UploadStatus
}

// ListRecordsInput описывает входные данные сценария получения списка приватных записей.
type ListRecordsInput struct {
	UserID uuid.UUID
}

// ListRecordsOutput описывает результат получения списка приватных записей.
type ListRecordsOutput struct {
	Items []model.RecordListItem
}

// GetRecordInput описывает входные данные сценария получения приватной записи.
type GetRecordInput struct {
	RecordID uuid.UUID
	UserID   uuid.UUID
}

// GetRecordOutput описывает результат получения приватной записи.
type GetRecordOutput struct {
	Record model.Record
}

// UpdateRecordInput описывает входные данные сценария обновления приватной записи.
type UpdateRecordInput struct {
	RecordID         uuid.UUID
	UserID           uuid.UUID
	Title            string
	Description      string
	EncryptedDEK     []byte
	EncryptedPayload []byte
	ExpectedVersion  int64
}

// UpdateRecordOutput описывает результат обновления приватной записи.
type UpdateRecordOutput struct {
	RecordID uuid.UUID
	Version  int64
}

// DeleteRecordInput описывает входные данные сценария удаления приватной записи.
type DeleteRecordInput struct {
	RecordID uuid.UUID
	UserID   uuid.UUID
}

// DeleteRecordOutput описывает результат удаления приватной записи.
type DeleteRecordOutput struct {
	RecordID uuid.UUID
}

// DownloadFileInput описывает входные данные сценария скачивания зашифрованного файла.
type DownloadFileInput struct {
	UserID   uuid.UUID
	RecordID uuid.UUID
}

// DownloadFileOutput описывает результат сценария скачивания зашифрованного файла.
type DownloadFileOutput struct {
	EncryptedFile io.ReadCloser
}

type recordRepository interface {
	Create(ctx context.Context, record model.Record) error
	GetByIDAndUserID(ctx context.Context, recordID uuid.UUID, userID uuid.UUID) (model.Record, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.RecordListItem, error)
	Update(ctx context.Context, record model.Record, expectedVersion int64) (int64, error)
	Delete(ctx context.Context, recordID uuid.UUID, userID uuid.UUID, deletedAt time.Time) error
}

type recordFileRepository interface {
	Create(ctx context.Context, file model.RecordFile) error
	Replace(ctx context.Context, file model.RecordFile) error
	UpdateUploadStatus(ctx context.Context, fileID uuid.UUID, status model.UploadStatus, updatedAt time.Time) error
}

type fileStorage interface {
	ObjectKey(userID, recordID, fileID uuid.UUID) string
	Put(ctx context.Context, objectKey string, data io.Reader, size int64) (int64, error)
	Get(ctx context.Context, objectKey string) (io.ReadCloser, error)
}

// RecordUseCase реализует сценарии работы с приватными записями.
type RecordUseCase struct {
	recordRepo     recordRepository
	recordFileRepo recordFileRepository
	fileStorage    fileStorage
	transactor     transactor
}

// NewRecordUseCase создает RecordUseCase.
func NewRecordUseCase(
	recordRepo recordRepository,
	recordFileRepo recordFileRepository,
	fileStorage fileStorage,
	transactor transactor,
) (*RecordUseCase, error) {
	if recordRepo == nil {
		return nil, errors.New("record repository is not provided")
	}
	if recordFileRepo == nil {
		return nil, errors.New("record file repository is not provided")
	}
	if fileStorage == nil {
		return nil, errors.New("file storage is not provided")
	}
	if transactor == nil {
		return nil, errors.New("transactor is not provided")
	}

	return &RecordUseCase{
		recordRepo:     recordRepo,
		recordFileRepo: recordFileRepo,
		fileStorage:    fileStorage,
		transactor:     transactor,
	}, nil
}

// CreateRecord создает приватную запись.
// Бинарные приватные записи создаются отдельным сценарием вместе с загрузкой файла.
func (uc *RecordUseCase) CreateRecord(ctx context.Context, in CreateRecordInput) (CreateRecordOutput, error) {
	if in.Type == model.RecordTypeBinary {
		return CreateRecordOutput{}, ErrBinaryRecordNotSupported
	}

	recordID, err := uuid.NewV7()
	if err != nil {
		return CreateRecordOutput{}, fmt.Errorf("failed to generate ID for record: %w", err)
	}

	now := time.Now().UTC()
	record := model.Record{
		ID:               recordID,
		UserID:           in.UserID,
		Type:             in.Type,
		Title:            in.Title,
		Description:      in.Description,
		EncryptedDEK:     model.EncryptedBlob{Data: in.EncryptedDEK},
		EncryptedPayload: model.EncryptedBlob{Data: in.EncryptedPayload},
		Version:          initialRecordVersion,
		CreatedAt:        now,
		UpdatedAt:        now,
		DeletedAt:        nil,
	}
	if err = uc.recordRepo.Create(ctx, record); err != nil {
		return CreateRecordOutput{}, fmt.Errorf("failed to create record: %w", err)
	}
	return CreateRecordOutput{RecordID: record.ID, Version: record.Version}, nil
}

// CreateBinaryRecord создает бинарную приватную запись вместе с загрузкой зашифрованного файла.
func (uc *RecordUseCase) CreateBinaryRecord(
	ctx context.Context,
	in CreateBinaryRecordInput,
) (CreateBinaryRecordOutput, error) {
	recordID, err := uuid.NewV7()
	if err != nil {
		return CreateBinaryRecordOutput{}, fmt.Errorf("failed to generate ID for record: %w", err)
	}
	fileID, err := uuid.NewV7()
	if err != nil {
		return CreateBinaryRecordOutput{}, fmt.Errorf("failed to generate ID for record file: %w", err)
	}

	now := time.Now().UTC()
	encryptedSize := in.EncryptedSize
	uploadMode := in.UploadMode
	record := model.Record{
		ID:               recordID,
		UserID:           in.UserID,
		Type:             model.RecordTypeBinary,
		Title:            in.Title,
		Description:      in.Description,
		EncryptedDEK:     model.EncryptedBlob{Data: in.EncryptedDEK},
		EncryptedPayload: model.EncryptedBlob{Data: in.EncryptedPayload},
		Version:          initialRecordVersion,
		CreatedAt:        now,
		UpdatedAt:        now,
		DeletedAt:        nil,
	}

	objectKey := uc.fileStorage.ObjectKey(in.UserID, recordID, fileID)
	file := model.RecordFile{
		ID:            fileID,
		RecordID:      recordID,
		ObjectKey:     objectKey,
		EncryptedSize: &encryptedSize,
		UploadMode:    &uploadMode,
		UploadStatus:  model.UploadStatusUploading,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err = uc.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		if err = uc.recordRepo.Create(ctx, record); err != nil {
			return fmt.Errorf("failed to persist record: %w", err)
		}
		if err = uc.recordFileRepo.Create(ctx, file); err != nil {
			return fmt.Errorf("failed to persist record file: %w", err)
		}
		return nil
	}); err != nil {
		return CreateBinaryRecordOutput{}, fmt.Errorf("failed to create binary record: %w", err)
	}

	written, err := uc.fileStorage.Put(ctx, objectKey, in.EncryptedFile, in.EncryptedSize)
	now = time.Now().UTC()
	if err != nil {
		statusErr := uc.recordFileRepo.UpdateUploadStatus(ctx, file.ID, model.UploadStatusFailed, now)
		if statusErr != nil {
			return CreateBinaryRecordOutput{}, fmt.Errorf(
				"failed to mark binary record upload as failed after upload error %q: %w",
				err.Error(),
				statusErr,
			)
		}

		if errors.Is(err, ErrBinaryEncryptedSizeMismatch) {
			return CreateBinaryRecordOutput{}, err
		}

		return CreateBinaryRecordOutput{}, fmt.Errorf("failed to upload record file: %w", err)
	}
	if written != in.EncryptedSize {
		statusErr := uc.recordFileRepo.UpdateUploadStatus(ctx, file.ID, model.UploadStatusFailed, now)
		if statusErr != nil {
			return CreateBinaryRecordOutput{}, fmt.Errorf(
				"failed to mark binary record upload as failed after encrypted size mismatch: %w",
				statusErr,
			)
		}

		return CreateBinaryRecordOutput{}, ErrBinaryEncryptedSizeMismatch
	}
	if err = uc.recordFileRepo.UpdateUploadStatus(ctx, file.ID, model.UploadStatusUploaded, now); err != nil {
		return CreateBinaryRecordOutput{}, fmt.Errorf("failed to mark binary record upload as uploaded: %w", err)
	}

	return CreateBinaryRecordOutput{
		RecordID:     recordID,
		Version:      initialRecordVersion,
		UploadStatus: model.UploadStatusUploaded,
	}, nil
}

// UpdateBinaryRecord обновляет бинарную приватную запись вместе с заменой зашифрованного файла.
func (uc *RecordUseCase) UpdateBinaryRecord(
	ctx context.Context,
	in UpdateBinaryRecordInput,
) (UpdateBinaryRecordOutput, error) {
	currentRecord, err := uc.recordRepo.GetByIDAndUserID(ctx, in.RecordID, in.UserID)
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			return UpdateBinaryRecordOutput{}, ErrRecordNotFound
		}
		return UpdateBinaryRecordOutput{}, fmt.Errorf("failed to get record for binary update: %w", err)
	}
	if currentRecord.Type != model.RecordTypeBinary {
		return UpdateBinaryRecordOutput{}, ErrRecordIsNotBinary
	}
	if currentRecord.File == nil {
		return UpdateBinaryRecordOutput{}, ErrRecordFileIsNotUploaded
	}

	fileID, err := uuid.NewV7()
	if err != nil {
		return UpdateBinaryRecordOutput{}, fmt.Errorf("failed to generate ID for record file: %w", err)
	}

	now := time.Now().UTC()
	encryptedSize := in.EncryptedSize
	uploadMode := in.UploadMode
	record := model.Record{
		ID:               in.RecordID,
		UserID:           in.UserID,
		Title:            in.Title,
		Description:      in.Description,
		EncryptedDEK:     model.EncryptedBlob{Data: in.EncryptedDEK},
		EncryptedPayload: model.EncryptedBlob{Data: in.EncryptedPayload},
		UpdatedAt:        now,
	}
	objectKey := uc.fileStorage.ObjectKey(in.UserID, in.RecordID, fileID)
	file := model.RecordFile{
		ID:            fileID,
		RecordID:      in.RecordID,
		ObjectKey:     objectKey,
		EncryptedSize: &encryptedSize,
		UploadMode:    &uploadMode,
		UploadStatus:  model.UploadStatusUploading,
		CreatedAt:     currentRecord.File.CreatedAt,
		UpdatedAt:     now,
	}

	var version int64
	if err = uc.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		version, err = uc.recordRepo.Update(ctx, record, in.ExpectedVersion)
		if err != nil {
			return err
		}
		if err = uc.recordFileRepo.Replace(ctx, file); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if errors.Is(err, ErrRecordNotFound) ||
			errors.Is(err, ErrRecordVersionConflict) ||
			errors.Is(err, ErrRecordIsNotBinary) {
			return UpdateBinaryRecordOutput{}, err
		}
		return UpdateBinaryRecordOutput{}, fmt.Errorf("failed to update binary record metadata: %w", err)
	}

	written, err := uc.fileStorage.Put(ctx, objectKey, in.EncryptedFile, in.EncryptedSize)
	now = time.Now().UTC()
	if err != nil {
		statusErr := uc.recordFileRepo.UpdateUploadStatus(ctx, file.ID, model.UploadStatusFailed, now)
		if statusErr != nil {
			return UpdateBinaryRecordOutput{}, fmt.Errorf(
				"failed to mark binary record upload as failed after upload error %q: %w",
				err.Error(),
				statusErr,
			)
		}

		if errors.Is(err, ErrBinaryEncryptedSizeMismatch) {
			return UpdateBinaryRecordOutput{}, err
		}

		return UpdateBinaryRecordOutput{}, fmt.Errorf("failed to upload record file: %w", err)
	}
	if written != in.EncryptedSize {
		statusErr := uc.recordFileRepo.UpdateUploadStatus(ctx, file.ID, model.UploadStatusFailed, now)
		if statusErr != nil {
			return UpdateBinaryRecordOutput{}, fmt.Errorf(
				"failed to mark binary record upload as failed after encrypted size mismatch: %w",
				statusErr,
			)
		}

		return UpdateBinaryRecordOutput{}, ErrBinaryEncryptedSizeMismatch
	}
	if err = uc.recordFileRepo.UpdateUploadStatus(ctx, file.ID, model.UploadStatusUploaded, now); err != nil {
		return UpdateBinaryRecordOutput{}, fmt.Errorf("failed to mark binary record upload as uploaded: %w", err)
	}

	return UpdateBinaryRecordOutput{
		RecordID:     in.RecordID,
		Version:      version,
		UploadStatus: model.UploadStatusUploaded,
	}, nil
}

// ListRecords возвращает список приватных записей.
func (uc *RecordUseCase) ListRecords(ctx context.Context, in ListRecordsInput) (ListRecordsOutput, error) {
	items, err := uc.recordRepo.ListByUserID(ctx, in.UserID)
	if err != nil {
		return ListRecordsOutput{}, fmt.Errorf("failed to list records: %w", err)
	}
	return ListRecordsOutput{Items: items}, nil
}

// GetRecord возвращает приватную запись.
func (uc *RecordUseCase) GetRecord(ctx context.Context, in GetRecordInput) (GetRecordOutput, error) {
	record, err := uc.recordRepo.GetByIDAndUserID(ctx, in.RecordID, in.UserID)
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			return GetRecordOutput{}, ErrRecordNotFound
		}
		return GetRecordOutput{}, fmt.Errorf("failed to get record: %w", err)
	}
	return GetRecordOutput{Record: record}, nil
}

// UpdateRecord обновляет приватную запись с проверкой ожидаемой версии.
func (uc *RecordUseCase) UpdateRecord(ctx context.Context, in UpdateRecordInput) (UpdateRecordOutput, error) {
	now := time.Now().UTC()
	record := model.Record{
		ID:               in.RecordID,
		UserID:           in.UserID,
		Title:            in.Title,
		Description:      in.Description,
		EncryptedDEK:     model.EncryptedBlob{Data: in.EncryptedDEK},
		EncryptedPayload: model.EncryptedBlob{Data: in.EncryptedPayload},
		UpdatedAt:        now,
	}
	version, err := uc.recordRepo.Update(ctx, record, in.ExpectedVersion)
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) || errors.Is(err, ErrRecordVersionConflict) {
			return UpdateRecordOutput{}, err
		}
		return UpdateRecordOutput{}, fmt.Errorf("failed to update record: %w", err)
	}
	return UpdateRecordOutput{RecordID: in.RecordID, Version: version}, nil
}

// DeleteRecord удаляет приватную запись.
func (uc *RecordUseCase) DeleteRecord(ctx context.Context, in DeleteRecordInput) (DeleteRecordOutput, error) {
	now := time.Now().UTC()
	if err := uc.recordRepo.Delete(ctx, in.RecordID, in.UserID, now); err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			return DeleteRecordOutput{}, ErrRecordNotFound
		}
		return DeleteRecordOutput{}, fmt.Errorf("failed to delete record: %w", err)
	}
	return DeleteRecordOutput{RecordID: in.RecordID}, nil
}

// DownloadFile возвращает поток зашифрованного файла приватной записи.
func (uc *RecordUseCase) DownloadFile(ctx context.Context, in DownloadFileInput) (DownloadFileOutput, error) {
	record, err := uc.recordRepo.GetByIDAndUserID(ctx, in.RecordID, in.UserID)
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			return DownloadFileOutput{}, ErrRecordNotFound
		}
		return DownloadFileOutput{}, fmt.Errorf("failed to get record for file download: %w", err)
	}
	if record.Type != model.RecordTypeBinary {
		return DownloadFileOutput{}, ErrRecordIsNotBinary
	}
	if record.File == nil || record.File.UploadStatus != model.UploadStatusUploaded {
		return DownloadFileOutput{}, ErrRecordFileIsNotUploaded
	}

	encryptedFile, err := uc.fileStorage.Get(ctx, record.File.ObjectKey)
	if err != nil {
		return DownloadFileOutput{}, fmt.Errorf("failed to get record file: %w", err)
	}
	return DownloadFileOutput{EncryptedFile: encryptedFile}, nil
}
