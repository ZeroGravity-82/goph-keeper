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

const (
	initialRecordVersion int64 = 1
	maxEncryptedFileSize       = 200 * 1024 * 1024
)

// MaxEncryptedFileSize возвращает максимальный размер зашифрованного файла для MVP-сценария.
func MaxEncryptedFileSize() int {
	return maxEncryptedFileSize
}

// CreateRecordInput описывает входные данные сценария создания записи пользователя.
type CreateRecordInput struct {
	UserID           uuid.UUID
	Type             model.RecordType
	Title            string
	Description      string
	EncryptedDEK     []byte
	EncryptedPayload []byte
}

// CreateRecordOutput описывает результат создания записи пользователя.
type CreateRecordOutput struct {
	RecordID uuid.UUID
	Version  int64
}

// CreateBinaryRecordInput описывает входные данные сценария создания бинарной записи пользователя.
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

// CreateBinaryRecordOutput описывает результат создания бинарной записи пользователя.
type CreateBinaryRecordOutput struct {
	RecordID     uuid.UUID
	Version      int64
	UploadStatus model.UploadStatus
}

// ListRecordsInput описывает входные данные сценария получения списка записей пользователя.
type ListRecordsInput struct {
	UserID uuid.UUID
}

// ListRecordsOutput описывает результат получения списка записей пользователя.
type ListRecordsOutput struct {
	Items []model.RecordListItem
}

// GetRecordInput описывает входные данные сценария получения записи пользователя.
type GetRecordInput struct {
	RecordID uuid.UUID
	UserID   uuid.UUID
}

// GetRecordOutput описывает результат получения записи пользователя.
type GetRecordOutput struct {
	Record model.Record
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
}

type recordFileRepository interface {
	Create(ctx context.Context, file model.RecordFile) error
	UpdateUploadStatus(ctx context.Context, fileID uuid.UUID, status model.UploadStatus, updatedAt time.Time) error
}

type fileStorage interface {
	ObjectKey(userID, recordID, fileID uuid.UUID) string
	Put(ctx context.Context, objectKey string, data io.Reader, size int64) (int64, error)
	Get(ctx context.Context, objectKey string) (io.ReadCloser, error)
}

// RecordUseCase реализует сценарии работы с записями пользователя.
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

// CreateRecord создает запись пользователя.
// Бинарные записи пользователя создаются отдельным сценарием вместе с загрузкой файла.
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

// CreateBinaryRecord создает бинарную запись пользователя вместе с загрузкой зашифрованного файла.
func (uc *RecordUseCase) CreateBinaryRecord(
	ctx context.Context,
	in CreateBinaryRecordInput,
) (CreateBinaryRecordOutput, error) {
	if in.UploadMode != model.UploadModeSinglePart {
		return CreateBinaryRecordOutput{}, ErrUploadModeNotSupported
	}
	if in.EncryptedSize <= 0 || in.EncryptedSize > maxEncryptedFileSize {
		return CreateBinaryRecordOutput{}, ErrInvalidBinaryEncryptedSize
	}

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

// ListRecords возвращает список записей пользователя.
func (uc *RecordUseCase) ListRecords(ctx context.Context, in ListRecordsInput) (ListRecordsOutput, error) {
	items, err := uc.recordRepo.ListByUserID(ctx, in.UserID)
	if err != nil {
		return ListRecordsOutput{}, fmt.Errorf("failed to list records: %w", err)
	}
	return ListRecordsOutput{Items: items}, nil
}

// GetRecord возвращает запись пользователя.
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

// DownloadFile возвращает поток зашифрованного файла записи пользователя.
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
