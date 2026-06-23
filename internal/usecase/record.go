package usecase

import (
	"context"
	"errors"
	"fmt"
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

type recordRepository interface {
	Create(ctx context.Context, record model.Record) error
	GetByIDAndUserID(ctx context.Context, recordID uuid.UUID, userID uuid.UUID) (model.Record, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.RecordListItem, error)
}

type recordFileRepository interface {
	Create(ctx context.Context, file model.RecordFile) error
}

// RecordUseCase реализует сценарии работы с приватными записями.
type RecordUseCase struct {
	recordRepo     recordRepository
	recordFileRepo recordFileRepository
	transactor     transactor
}

// NewRecordUseCase создает RecordUseCase.
func NewRecordUseCase(
	recordRepo recordRepository,
	recordFileRepo recordFileRepository,
	transactor transactor,
) (*RecordUseCase, error) {
	if recordRepo == nil {
		return nil, errors.New("record repository is not provided")
	}
	if recordFileRepo == nil {
		return nil, errors.New("record file repository is not provided")
	}
	if transactor == nil {
		return nil, errors.New("transactor is not provided")
	}

	return &RecordUseCase{
		recordRepo:     recordRepo,
		recordFileRepo: recordFileRepo,
		transactor:     transactor,
	}, nil
}

// CreateRecord создает приватную запись пользователя.
func (uc *RecordUseCase) CreateRecord(ctx context.Context, in CreateRecordInput) (CreateRecordOutput, error) {
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

	if err = uc.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		if err = uc.recordRepo.Create(ctx, record); err != nil {
			return fmt.Errorf("failed to persist record: %w", err)
		}
		if record.Type != model.RecordTypeBinary {
			return nil
		}

		fileID, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("failed to generate ID for record file: %w", err)
		}
		file := model.RecordFile{
			ID:            fileID,
			RecordID:      record.ID,
			ObjectKey:     buildObjectKey(in.UserID, recordID, fileID),
			EncryptedSize: nil,
			UploadMode:    nil,
			UploadStatus:  model.UploadStatusPending,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err = uc.recordFileRepo.Create(ctx, file); err != nil {
			return fmt.Errorf("failed to persist record file: %w", err)
		}
		return nil
	}); err != nil {
		return CreateRecordOutput{}, fmt.Errorf("failed to create record: %w", err)
	}

	return CreateRecordOutput{RecordID: record.ID, Version: record.Version}, nil
}

// ListRecords возвращает список приватных записей пользователя.
func (uc *RecordUseCase) ListRecords(ctx context.Context, in ListRecordsInput) (ListRecordsOutput, error) {
	items, err := uc.recordRepo.ListByUserID(ctx, in.UserID)
	if err != nil {
		return ListRecordsOutput{}, fmt.Errorf("failed to list records: %w", err)
	}
	return ListRecordsOutput{Items: items}, nil
}

// GetRecord возвращает приватную запись пользователя.
func (uc *RecordUseCase) GetRecord(ctx context.Context, in GetRecordInput) (GetRecordOutput, error) {
	record, err := uc.recordRepo.GetByIDAndUserID(ctx, in.RecordID, in.UserID)
	if err != nil {
		if errors.Is(err, model.ErrRecordNotFound) {
			return GetRecordOutput{}, model.ErrRecordNotFound
		}
		return GetRecordOutput{}, fmt.Errorf("failed to get record: %w", err)
	}
	return GetRecordOutput{Record: record}, nil
}

func buildObjectKey(userID, recordID, fileID uuid.UUID) string {
	return fmt.Sprintf("users/%s/records/%s/files/%s/payload", userID, recordID, fileID)
}
