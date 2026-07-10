package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
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

const (
	// MultipartUploadStatusUploading обозначает активную multipart-загрузку.
	MultipartUploadStatusUploading MultipartUploadStatus = "uploading"
	// MultipartUploadStatusCompleted обозначает успешно завершенную multipart-загрузку.
	MultipartUploadStatusCompleted MultipartUploadStatus = "completed"
	// MultipartUploadStatusAborted обозначает отмененную multipart-загрузку.
	MultipartUploadStatusAborted MultipartUploadStatus = "aborted"
)

// MultipartUploadStatus представляет статус возобновляемой multipart-загрузки файла.
type MultipartUploadStatus string

// MultipartUpload описывает серверную сессию возобновляемой multipart-загрузки файла.
type MultipartUpload struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	RecordID        uuid.UUID
	RecordVersion   int64
	FileID          uuid.UUID
	ObjectKey       string
	StorageUploadID string
	EncryptedSize   int64
	PartSize        int64
	Status          MultipartUploadStatus
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CompletedAt     *time.Time
}

// MultipartUploadPart описывает загруженную часть multipart-загрузки.
type MultipartUploadPart struct {
	UploadID   uuid.UUID
	PartNumber int32
	Size       int64
	ETag       string
	CreatedAt  time.Time
}

// MultipartUploadPartOutput описывает загруженную часть multipart-загрузки.
type MultipartUploadPartOutput struct {
	PartNumber int32
	Size       int64
	ETag       string
}

// StartBinaryMultipartUploadInput описывает входные данные сценария начала multipart-загрузки бинарной приватной
// записи.
type StartBinaryMultipartUploadInput struct {
	UserID           uuid.UUID
	RecordID         uuid.UUID
	ExpectedVersion  int64
	Title            string
	Description      string
	EncryptedDEK     []byte
	EncryptedPayload []byte
	EncryptedSize    int64
	PartSize         int64
}

// StartBinaryMultipartUploadOutput описывает результат начала multipart-загрузки бинарной приватной записи.
type StartBinaryMultipartUploadOutput struct {
	UploadID      uuid.UUID
	RecordID      uuid.UUID
	Version       int64
	PartSize      int64
	UploadStatus  model.UploadStatus
	UploadedParts []MultipartUploadPartOutput
}

// GetBinaryMultipartUploadStatusInput описывает входные данные сценария получения статуса multipart-загрузки.
type GetBinaryMultipartUploadStatusInput struct {
	UserID   uuid.UUID
	UploadID uuid.UUID
}

// GetBinaryMultipartUploadStatusOutput описывает состояние multipart-загрузки бинарной приватной записи.
type GetBinaryMultipartUploadStatusOutput struct {
	UploadID      uuid.UUID
	RecordID      uuid.UUID
	Version       int64
	EncryptedSize int64
	PartSize      int64
	UploadStatus  model.UploadStatus
	UploadedParts []MultipartUploadPartOutput
}

// UploadBinaryMultipartPartInput описывает входные данные сценария загрузки одной части файла.
type UploadBinaryMultipartPartInput struct {
	UserID     uuid.UUID
	UploadID   uuid.UUID
	PartNumber int32
	PartSize   int64
	Data       io.Reader
}

// UploadBinaryMultipartPartOutput описывает результат загрузки одной части файла.
type UploadBinaryMultipartPartOutput struct {
	UploadID uuid.UUID
	Part     MultipartUploadPartOutput
}

// CompleteBinaryMultipartUploadInput описывает входные данные сценария завершения multipart-загрузки.
type CompleteBinaryMultipartUploadInput struct {
	UserID          uuid.UUID
	UploadID        uuid.UUID
	EncryptedSHA256 string
}

// CompleteBinaryMultipartUploadOutput описывает результат завершения multipart-загрузки.
type CompleteBinaryMultipartUploadOutput struct {
	RecordID        uuid.UUID
	Version         int64
	UploadStatus    model.UploadStatus
	EncryptedSHA256 string
}

// AbortBinaryMultipartUploadInput описывает входные данные сценария отмены multipart-загрузки.
type AbortBinaryMultipartUploadInput struct {
	UserID   uuid.UUID
	UploadID uuid.UUID
}

// AbortBinaryMultipartUploadOutput описывает результат отмены multipart-загрузки.
type AbortBinaryMultipartUploadOutput struct {
	UploadID     uuid.UUID
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

// recordRepository описывает операции с приватными записями, которые нужны сценариям записей.
type recordRepository interface {
	Create(ctx context.Context, record model.Record) error
	GetByIDAndUserID(ctx context.Context, recordID uuid.UUID, userID uuid.UUID) (model.Record, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.RecordListItem, error)
	Update(ctx context.Context, record model.Record, expectedVersion int64) (int64, error)
	Delete(ctx context.Context, recordID uuid.UUID, userID uuid.UUID, deletedAt time.Time) error
}

// recordFileRepository описывает операции с метаданными файла бинарной приватной записи.
type recordFileRepository interface {
	Create(ctx context.Context, file model.RecordFile) error
	Replace(ctx context.Context, file model.RecordFile) error
	UpdateUploadStatus(ctx context.Context, fileID uuid.UUID, status model.UploadStatus, updatedAt time.Time) error
	CompleteUpload(ctx context.Context, fileID uuid.UUID, encryptedSHA256 string, updatedAt time.Time) error
}

// multipartUploadRepository описывает операции с состоянием возобновляемой multipart-загрузки файла.
type multipartUploadRepository interface {
	Create(ctx context.Context, upload MultipartUpload) error
	GetByIDAndUserID(ctx context.Context, uploadID uuid.UUID, userID uuid.UUID) (MultipartUpload, error)
	UpsertPart(ctx context.Context, part MultipartUploadPart) error
	ListParts(ctx context.Context, uploadID uuid.UUID) ([]MultipartUploadPart, error)
	UpdateStatus(
		ctx context.Context,
		uploadID uuid.UUID,
		status MultipartUploadStatus,
		updatedAt time.Time,
		completedAt *time.Time,
	) error
}

// fileStorage описывает операции с объектным хранилищем зашифрованных файлов.
type fileStorage interface {
	ObjectKey(userID, recordID, fileID uuid.UUID) string
	Get(ctx context.Context, objectKey string) (io.ReadCloser, error)
	CreateMultipartUpload(ctx context.Context, objectKey string) (string, error)
	PutMultipartPart(
		ctx context.Context,
		objectKey string,
		uploadID string,
		partNumber int32,
		data io.Reader,
		size int64,
	) (MultipartUploadPart, error)
	CompleteMultipartUpload(ctx context.Context, objectKey string, uploadID string, parts []MultipartUploadPart) error
	AbortMultipartUpload(ctx context.Context, objectKey string, uploadID string) error
}

// RecordUseCase реализует сценарии работы с приватными записями.
type RecordUseCase struct {
	recordRepo     recordRepository
	recordFileRepo recordFileRepository
	multipartRepo  multipartUploadRepository
	fileStorage    fileStorage
	transactor     transactor
	mutationGuard  recordMutationGuard
}

// NewRecordUseCase создает RecordUseCase.
func NewRecordUseCase(
	recordRepo recordRepository,
	recordFileRepo recordFileRepository,
	multipartRepo multipartUploadRepository,
	fileStorage fileStorage,
	transactor transactor,
	mutationGuard recordMutationGuard,
) (*RecordUseCase, error) {
	if recordRepo == nil {
		return nil, errors.New("record repository is not provided")
	}
	if recordFileRepo == nil {
		return nil, errors.New("record file repository is not provided")
	}
	if multipartRepo == nil {
		return nil, errors.New("multipart upload repository is not provided")
	}
	if fileStorage == nil {
		return nil, errors.New("file storage is not provided")
	}
	if transactor == nil {
		return nil, errors.New("transactor is not provided")
	}
	if mutationGuard == nil {
		return nil, errors.New("record mutation guard is not provided")
	}

	return &RecordUseCase{
		recordRepo:     recordRepo,
		recordFileRepo: recordFileRepo,
		multipartRepo:  multipartRepo,
		fileStorage:    fileStorage,
		transactor:     transactor,
		mutationGuard:  mutationGuard,
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

	if err = uc.mutationGuard.WithUserRecordsLock(ctx, in.UserID, func(ctx context.Context) error {
		if err = uc.recordRepo.Create(ctx, record); err != nil {
			return fmt.Errorf("failed to create record: %w", err)
		}
		return nil
	}); err != nil {
		return CreateRecordOutput{}, err
	}
	return CreateRecordOutput{RecordID: record.ID, Version: record.Version}, nil
}

// StartBinaryMultipartUpload создает или обновляет бинарную приватную запись и открывает сессию multipart-загрузки
// файла.
func (uc *RecordUseCase) StartBinaryMultipartUpload(
	ctx context.Context,
	in StartBinaryMultipartUploadInput,
) (StartBinaryMultipartUploadOutput, error) {
	fileID, err := uuid.NewV7()
	if err != nil {
		return StartBinaryMultipartUploadOutput{}, fmt.Errorf("failed to generate ID for record file: %w", err)
	}
	uploadID, err := uuid.NewV7()
	if err != nil {
		return StartBinaryMultipartUploadOutput{}, fmt.Errorf("failed to generate ID for multipart upload: %w", err)
	}

	recordID := in.RecordID
	recordVersion := initialRecordVersion
	// Пустой RecordID означает создание новой бинарной записи; непустой RecordID открывает загрузку замены файла.
	if recordID == uuid.Nil {
		recordID, err = uuid.NewV7()
		if err != nil {
			return StartBinaryMultipartUploadOutput{}, fmt.Errorf("failed to generate ID for record: %w", err)
		}
	} else {
		currentRecord, getErr := uc.recordRepo.GetByIDAndUserID(ctx, in.RecordID, in.UserID)
		if getErr != nil {
			if errors.Is(getErr, ErrRecordNotFound) {
				return StartBinaryMultipartUploadOutput{}, ErrRecordNotFound
			}
			return StartBinaryMultipartUploadOutput{}, fmt.Errorf("failed to get record for binary update: %w", getErr)
		}
		if currentRecord.Type != model.RecordTypeBinary {
			return StartBinaryMultipartUploadOutput{}, ErrRecordIsNotBinary
		}
		if currentRecord.File == nil {
			return StartBinaryMultipartUploadOutput{}, ErrRecordFileIsNotUploaded
		}
	}

	objectKey := uc.fileStorage.ObjectKey(in.UserID, recordID, fileID)
	storageUploadID, err := uc.fileStorage.CreateMultipartUpload(ctx, objectKey)
	if err != nil {
		return StartBinaryMultipartUploadOutput{}, fmt.Errorf(
			"failed to create multipart upload in file storage: %w",
			err,
		)
	}

	now := time.Now().UTC()
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
	}
	file := model.RecordFile{
		ID:            fileID,
		RecordID:      recordID,
		ObjectKey:     objectKey,
		EncryptedSize: new(in.EncryptedSize),
		UploadStatus:  model.UploadStatusUploading,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	upload := MultipartUpload{
		ID:              uploadID,
		UserID:          in.UserID,
		RecordID:        recordID,
		RecordVersion:   recordVersion,
		FileID:          fileID,
		ObjectKey:       objectKey,
		StorageUploadID: storageUploadID,
		EncryptedSize:   in.EncryptedSize,
		PartSize:        in.PartSize,
		Status:          MultipartUploadStatusUploading,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err = uc.mutationGuard.WithUserRecordsLock(ctx, in.UserID, func(ctx context.Context) error {
		if in.RecordID == uuid.Nil {
			if err = uc.recordRepo.Create(ctx, record); err != nil {
				return fmt.Errorf("failed to persist record: %w", err)
			}
			if err = uc.recordFileRepo.Create(ctx, file); err != nil {
				return fmt.Errorf("failed to persist record file: %w", err)
			}
		} else {
			recordVersion, err = uc.recordRepo.Update(ctx, record, in.ExpectedVersion)
			if err != nil {
				return err
			}
			upload.RecordVersion = recordVersion
			if err = uc.recordFileRepo.Replace(ctx, file); err != nil {
				return err
			}
		}
		if err = uc.multipartRepo.Create(ctx, upload); err != nil {
			return fmt.Errorf("failed to persist multipart upload: %w", err)
		}
		return nil
	}); err != nil {
		// Сессия в объектном хранилище создается до транзакции БД, поэтому при ошибке БД ее нужно закрыть отдельно.
		abortErr := uc.fileStorage.AbortMultipartUpload(ctx, objectKey, storageUploadID)
		if abortErr != nil {
			return StartBinaryMultipartUploadOutput{}, fmt.Errorf(
				"failed to rollback multipart upload after database error %q: %w",
				err.Error(),
				abortErr,
			)
		}
		return StartBinaryMultipartUploadOutput{}, fmt.Errorf("failed to start binary multipart upload: %w", err)
	}

	return StartBinaryMultipartUploadOutput{
		UploadID:     uploadID,
		RecordID:     recordID,
		Version:      recordVersion,
		PartSize:     in.PartSize,
		UploadStatus: model.UploadStatusUploading,
	}, nil
}

// GetBinaryMultipartUploadStatus возвращает состояние сессии multipart-загрузки и список уже загруженных частей.
func (uc *RecordUseCase) GetBinaryMultipartUploadStatus(
	ctx context.Context,
	in GetBinaryMultipartUploadStatusInput,
) (GetBinaryMultipartUploadStatusOutput, error) {
	upload, parts, err := uc.multipartUploadWithParts(ctx, in.UploadID, in.UserID)
	if err != nil {
		return GetBinaryMultipartUploadStatusOutput{}, err
	}
	return GetBinaryMultipartUploadStatusOutput{
		UploadID:      upload.ID,
		RecordID:      upload.RecordID,
		Version:       upload.RecordVersion,
		EncryptedSize: upload.EncryptedSize,
		PartSize:      upload.PartSize,
		UploadStatus:  uploadStatusFromMultipartStatus(upload.Status),
		UploadedParts: multipartPartOutputs(parts),
	}, nil
}

// UploadBinaryMultipartPart загружает или повторно загружает одну часть файла в активную сессию multipart-загрузки.
func (uc *RecordUseCase) UploadBinaryMultipartPart(
	ctx context.Context,
	in UploadBinaryMultipartPartInput,
) (UploadBinaryMultipartPartOutput, error) {
	if in.Data == nil {
		return UploadBinaryMultipartPartOutput{}, ErrMultipartUploadPartInvalid
	}

	upload, err := uc.multipartRepo.GetByIDAndUserID(ctx, in.UploadID, in.UserID)
	if err != nil {
		if errors.Is(err, ErrMultipartUploadNotFound) {
			return UploadBinaryMultipartPartOutput{}, ErrMultipartUploadNotFound
		}
		return UploadBinaryMultipartPartOutput{}, fmt.Errorf("failed to get multipart upload: %w", err)
	}
	if upload.Status != MultipartUploadStatusUploading {
		return UploadBinaryMultipartPartOutput{}, ErrMultipartUploadNotActive
	}
	// Клиент может повторять загрузку части, но номер и размер должны совпадать с разбиением исходного файла.
	expectedSize, err := expectedMultipartPartSize(upload, in.PartNumber)
	if err != nil {
		return UploadBinaryMultipartPartOutput{}, err
	}
	if in.PartSize != expectedSize {
		return UploadBinaryMultipartPartOutput{}, ErrMultipartUploadPartInvalid
	}

	part, err := uc.fileStorage.PutMultipartPart(
		ctx,
		upload.ObjectKey,
		upload.StorageUploadID,
		in.PartNumber,
		in.Data,
		in.PartSize,
	)
	if err != nil {
		return UploadBinaryMultipartPartOutput{}, fmt.Errorf("failed to upload multipart part: %w", err)
	}
	part.UploadID = upload.ID
	part.CreatedAt = time.Now().UTC()
	if err = uc.multipartRepo.UpsertPart(ctx, part); err != nil {
		return UploadBinaryMultipartPartOutput{}, fmt.Errorf("failed to persist multipart part: %w", err)
	}

	return UploadBinaryMultipartPartOutput{
		UploadID: upload.ID,
		Part:     multipartPartOutput(part),
	}, nil
}

// CompleteBinaryMultipartUpload завершает multipart-загрузку после получения всех частей файла.
func (uc *RecordUseCase) CompleteBinaryMultipartUpload(
	ctx context.Context,
	in CompleteBinaryMultipartUploadInput,
) (CompleteBinaryMultipartUploadOutput, error) {
	upload, parts, err := uc.multipartUploadWithParts(ctx, in.UploadID, in.UserID)
	if err != nil {
		return CompleteBinaryMultipartUploadOutput{}, err
	}
	if upload.Status != MultipartUploadStatusUploading {
		return CompleteBinaryMultipartUploadOutput{}, ErrMultipartUploadNotActive
	}
	// Перед закрытием multipart-загрузки проверяем, что сервер уже получил непрерывный набор частей полного размера.
	if err = validateCompleteMultipartParts(upload, parts); err != nil {
		return CompleteBinaryMultipartUploadOutput{}, err
	}

	if err = uc.fileStorage.CompleteMultipartUpload(ctx, upload.ObjectKey, upload.StorageUploadID, parts); err != nil {
		return CompleteBinaryMultipartUploadOutput{}, fmt.Errorf(
			"failed to complete multipart upload in file storage: %w",
			err,
		)
	}
	actualSHA256, err := uc.encryptedFileSHA256(ctx, upload.ObjectKey)
	if err != nil {
		return CompleteBinaryMultipartUploadOutput{}, err
	}
	if actualSHA256 != in.EncryptedSHA256 {
		if markErr := uc.markMultipartUploadFailed(ctx, upload); markErr != nil {
			return CompleteBinaryMultipartUploadOutput{}, fmt.Errorf(
				"failed to mark multipart upload as failed after checksum mismatch: %w",
				markErr,
			)
		}
		return CompleteBinaryMultipartUploadOutput{}, ErrMultipartUploadChecksumMismatch
	}

	now := time.Now().UTC()
	if err = uc.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		if err = uc.multipartRepo.UpdateStatus(
			ctx,
			upload.ID,
			MultipartUploadStatusCompleted,
			now,
			&now,
		); err != nil {
			return err
		}
		if err = uc.recordFileRepo.CompleteUpload(ctx, upload.FileID, in.EncryptedSHA256, now); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return CompleteBinaryMultipartUploadOutput{}, fmt.Errorf(
			"failed to mark multipart upload as completed: %w",
			err,
		)
	}

	return CompleteBinaryMultipartUploadOutput{
		RecordID:        upload.RecordID,
		Version:         upload.RecordVersion,
		UploadStatus:    model.UploadStatusUploaded,
		EncryptedSHA256: in.EncryptedSHA256,
	}, nil
}

// AbortBinaryMultipartUpload отменяет активную multipart-загрузку и помечает файл как неуспешно загруженный.
func (uc *RecordUseCase) AbortBinaryMultipartUpload(
	ctx context.Context,
	in AbortBinaryMultipartUploadInput,
) (AbortBinaryMultipartUploadOutput, error) {
	upload, err := uc.multipartRepo.GetByIDAndUserID(ctx, in.UploadID, in.UserID)
	if err != nil {
		if errors.Is(err, ErrMultipartUploadNotFound) {
			return AbortBinaryMultipartUploadOutput{}, ErrMultipartUploadNotFound
		}
		return AbortBinaryMultipartUploadOutput{}, fmt.Errorf("failed to get multipart upload: %w", err)
	}
	if upload.Status == MultipartUploadStatusCompleted {
		return AbortBinaryMultipartUploadOutput{}, ErrMultipartUploadNotActive
	}
	if upload.Status == MultipartUploadStatusUploading {
		if err = uc.fileStorage.AbortMultipartUpload(ctx, upload.ObjectKey, upload.StorageUploadID); err != nil {
			return AbortBinaryMultipartUploadOutput{}, fmt.Errorf(
				"failed to abort multipart upload in file storage: %w",
				err,
			)
		}
	}

	now := time.Now().UTC()
	if err = uc.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		if err = uc.multipartRepo.UpdateStatus(ctx, upload.ID, MultipartUploadStatusAborted, now, nil); err != nil {
			return err
		}
		if err = uc.recordFileRepo.UpdateUploadStatus(ctx, upload.FileID, model.UploadStatusFailed, now); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return AbortBinaryMultipartUploadOutput{}, fmt.Errorf("failed to mark multipart upload as aborted: %w", err)
	}

	return AbortBinaryMultipartUploadOutput{
		UploadID:     upload.ID,
		UploadStatus: model.UploadStatusFailed,
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
	var version int64
	err := uc.mutationGuard.WithUserRecordsLock(ctx, in.UserID, func(ctx context.Context) error {
		var updateErr error
		version, updateErr = uc.recordRepo.Update(ctx, record, in.ExpectedVersion)
		if updateErr != nil {
			return updateErr
		}
		return nil
	})
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
	err := uc.mutationGuard.WithUserRecordsLock(ctx, in.UserID, func(ctx context.Context) error {
		return uc.recordRepo.Delete(ctx, in.RecordID, in.UserID, now)
	})
	if err != nil {
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

// multipartUploadWithParts получает сессию multipart-загрузки вместе со списком уже сохраненных частей.
func (uc *RecordUseCase) multipartUploadWithParts(
	ctx context.Context,
	uploadID uuid.UUID,
	userID uuid.UUID,
) (MultipartUpload, []MultipartUploadPart, error) {
	upload, err := uc.multipartRepo.GetByIDAndUserID(ctx, uploadID, userID)
	if err != nil {
		if errors.Is(err, ErrMultipartUploadNotFound) {
			return MultipartUpload{}, nil, ErrMultipartUploadNotFound
		}
		return MultipartUpload{}, nil, fmt.Errorf("failed to get multipart upload: %w", err)
	}
	parts, err := uc.multipartRepo.ListParts(ctx, upload.ID)
	if err != nil {
		return MultipartUpload{}, nil, fmt.Errorf("failed to list multipart upload parts: %w", err)
	}
	return upload, parts, nil
}

// encryptedFileSHA256 рассчитывает SHA-256 зашифрованного файла потоковым чтением из объектного хранилища.
func (uc *RecordUseCase) encryptedFileSHA256(ctx context.Context, objectKey string) (string, error) {
	encryptedFile, err := uc.fileStorage.Get(ctx, objectKey)
	if err != nil {
		return "", fmt.Errorf("failed to get completed encrypted file for checksum: %w", err)
	}
	defer func() {
		_ = encryptedFile.Close()
	}()

	hash := sha256.New()
	if _, err = io.Copy(hash, encryptedFile); err != nil {
		return "", fmt.Errorf("failed to calculate encrypted file checksum: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// markMultipartUploadFailed переводит запись файла в failed, когда объект собран, но не прошел проверку целостности.
func (uc *RecordUseCase) markMultipartUploadFailed(ctx context.Context, upload MultipartUpload) error {
	now := time.Now().UTC()
	return uc.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := uc.multipartRepo.UpdateStatus(ctx, upload.ID, MultipartUploadStatusAborted, now, nil); err != nil {
			return err
		}
		if err := uc.recordFileRepo.UpdateUploadStatus(ctx, upload.FileID, model.UploadStatusFailed, now); err != nil {
			return err
		}
		return nil
	})
}

// expectedMultipartPartSize вычисляет ожидаемый размер части с учетом того, что последняя часть может быть короче.
func expectedMultipartPartSize(upload MultipartUpload, partNumber int32) (int64, error) {
	if partNumber <= 0 || upload.PartSize <= 0 || upload.EncryptedSize <= 0 {
		return 0, ErrMultipartUploadPartInvalid
	}
	offset := int64(partNumber-1) * upload.PartSize
	if offset >= upload.EncryptedSize {
		return 0, ErrMultipartUploadPartInvalid
	}
	remaining := upload.EncryptedSize - offset
	if remaining < upload.PartSize {
		return remaining, nil
	}
	return upload.PartSize, nil
}

// validateCompleteMultipartParts проверяет, что для завершения загрузки есть все части без пропусков и лишних байт.
func validateCompleteMultipartParts(upload MultipartUpload, parts []MultipartUploadPart) error {
	if len(parts) == 0 {
		return ErrMultipartUploadIncomplete
	}

	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})

	var total int64
	for i, part := range parts {
		expectedPartNumber := int32(i + 1)
		if part.PartNumber != expectedPartNumber {
			return ErrMultipartUploadIncomplete
		}
		expectedSize, err := expectedMultipartPartSize(upload, part.PartNumber)
		if err != nil {
			return err
		}
		if part.Size != expectedSize || part.ETag == "" {
			return ErrMultipartUploadIncomplete
		}
		total += part.Size
	}
	if total != upload.EncryptedSize {
		return ErrMultipartUploadIncomplete
	}
	return nil
}

// uploadStatusFromMultipartStatus переводит внутренний статус multipart-загрузки в пользовательский статус файла.
func uploadStatusFromMultipartStatus(status MultipartUploadStatus) model.UploadStatus {
	switch status {
	case MultipartUploadStatusCompleted:
		return model.UploadStatusUploaded
	case MultipartUploadStatusUploading:
		return model.UploadStatusUploading
	default:
		return model.UploadStatusFailed
	}
}

// multipartPartOutputs преобразует список частей из внутреннего формата usecase в выходной формат сценария.
func multipartPartOutputs(parts []MultipartUploadPart) []MultipartUploadPartOutput {
	out := make([]MultipartUploadPartOutput, 0, len(parts))
	for _, part := range parts {
		out = append(out, multipartPartOutput(part))
	}
	return out
}

// multipartPartOutput преобразует одну загруженную часть в выходной формат сценария.
func multipartPartOutput(part MultipartUploadPart) MultipartUploadPartOutput {
	return MultipartUploadPartOutput{
		PartNumber: part.PartNumber,
		Size:       part.Size,
		ETag:       part.ETag,
	}
}
