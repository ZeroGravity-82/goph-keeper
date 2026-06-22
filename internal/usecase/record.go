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

type recordRepository interface {
	Create(ctx context.Context, record model.Record) error
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

func buildObjectKey(userID, recordID, fileID uuid.UUID) string {
	return fmt.Sprintf("users/%s/records/%s/files/%s/payload", userID, recordID, fileID)
}
