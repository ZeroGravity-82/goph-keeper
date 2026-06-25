package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"zerogravity-82/goph-keeper/internal/domain/model"
	"zerogravity-82/goph-keeper/internal/storage/postgres/dto"
)

// RecordRepository реализует доступ к записям пользователя в PostgreSQL.
type RecordRepository struct {
	db *sqlx.DB
}

// NewRecordRepository создает RecordRepository на основе подключения к БД.
func NewRecordRepository(db *sqlx.DB) (*RecordRepository, error) {
	if db == nil {
		return nil, errors.New("database connection is not provided")
	}
	return &RecordRepository{db: db}, nil
}

// Create сохраняет новую запись пользователя.
func (r *RecordRepository) Create(ctx context.Context, record model.Record) error {
	const q = `
INSERT INTO record (
    id, app_user_id, type, title, description, encrypted_dek, encrypted_payload, version, created_at, updated_at, deleted_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
`
	exec := executorFromContext(ctx, r.db)
	_, err := exec.ExecContext(
		ctx,
		q,
		record.ID,
		record.UserID,
		string(record.Type),
		record.Title,
		record.Description,
		record.EncryptedDEK.Data,
		record.EncryptedPayload.Data,
		record.Version,
		record.CreatedAt,
		record.UpdatedAt,
		record.DeletedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to persist record: %w", err)
	}
	return nil
}

// GetByIDAndUserID возвращает запись пользователя по ID записи.
func (r *RecordRepository) GetByIDAndUserID(
	ctx context.Context,
	recordID uuid.UUID,
	userID uuid.UUID,
) (model.Record, error) {
	const q = `
SELECT
    r.id,
    r.app_user_id,
    r.type,
    r.title,
    r.description,
    r.encrypted_dek,
    r.encrypted_payload,
    r.version,
    r.created_at,
    r.updated_at,
    r.deleted_at,
    rf.id AS file_id,
    rf.record_id AS file_record_id,
    rf.object_key AS file_object_key,
    rf.encrypted_size AS file_encrypted_size,
    rf.upload_mode AS file_upload_mode,
    rf.upload_status AS file_upload_status,
    rf.created_at AS file_created_at,
    rf.updated_at AS file_updated_at
FROM record r
LEFT JOIN record_file rf ON rf.record_id = r.id
WHERE r.id = $1 AND r.app_user_id = $2 AND r.deleted_at IS NULL
`

	var row dto.RecordWithFile
	exec := executorFromContext(ctx, r.db)
	if err := exec.GetContext(ctx, &row, q, recordID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Record{}, model.ErrRecordNotFound
		}
		return model.Record{}, fmt.Errorf("failed to select record by ID and user ID: %w", err)
	}
	return recordWithFileFromDTO(row), nil
}

// ListByUserID возвращает список записей пользователя.
func (r *RecordRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.RecordListItem, error) {
	const q = `
SELECT r.id, r.type, r.title, r.description, r.created_at, r.updated_at, rf.upload_status
FROM record r
LEFT JOIN record_file rf ON rf.record_id = r.id
WHERE r.app_user_id = $1 AND r.deleted_at IS NULL
ORDER BY r.updated_at DESC, r.id DESC
`

	var rows []dto.RecordListItem
	exec := executorFromContext(ctx, r.db)
	if err := exec.SelectContext(ctx, &rows, q, userID); err != nil {
		return nil, fmt.Errorf("failed to select record list by user ID: %w", err)
	}

	items := make([]model.RecordListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, recordListItemFromDTO(row))
	}
	return items, nil
}

func recordListItemFromDTO(row dto.RecordListItem) model.RecordListItem {
	item := model.RecordListItem{
		ID:          row.ID,
		Type:        model.RecordType(row.Type),
		Title:       row.Title,
		Description: row.Description,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
	if row.UploadStatus != nil {
		item.File = &model.RecordListItemFile{UploadStatus: model.UploadStatus(*row.UploadStatus)}
	}
	return item
}

func recordWithFileFromDTO(row dto.RecordWithFile) model.Record {
	record := model.Record{
		ID:               row.ID,
		UserID:           row.UserID,
		Type:             model.RecordType(row.Type),
		Title:            row.Title,
		Description:      row.Description,
		EncryptedDEK:     model.EncryptedBlob{Data: row.EncryptedDEK},
		EncryptedPayload: model.EncryptedBlob{Data: row.EncryptedPayload},
		Version:          row.Version,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		DeletedAt:        row.DeletedAt,
	}
	if row.FileID != nil {
		record.File = &model.RecordFile{
			ID:            *row.FileID,
			RecordID:      *row.FileRecordID,
			ObjectKey:     *row.FileObjectKey,
			EncryptedSize: row.FileEncryptedSize,
			UploadMode:    uploadModeFromDB(row.FileUploadMode),
			UploadStatus:  model.UploadStatus(*row.FileUploadStatus),
			CreatedAt:     *row.FileCreatedAt,
			UpdatedAt:     *row.FileUpdatedAt,
		}
	}
	return record
}

// RecordFileRepository реализует доступ к техническим данным (ключ в объектном хранилище, размер зашифрованного файла,
// режим загрузки на сервер, статус загрузки на сервер) файлов записей пользователя в PostgreSQL.
type RecordFileRepository struct {
	db *sqlx.DB
}

// NewRecordFileRepository создает RecordFileRepository на основе подключения к БД.
func NewRecordFileRepository(db *sqlx.DB) (*RecordFileRepository, error) {
	if db == nil {
		return nil, errors.New("database connection is not provided")
	}
	return &RecordFileRepository{db: db}, nil
}

// Create сохраняет техническую информацию о файле записи пользователя.
func (r *RecordFileRepository) Create(ctx context.Context, file model.RecordFile) error {
	const q = `
INSERT INTO record_file (id, record_id, object_key, encrypted_size, upload_mode, upload_status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
`
	exec := executorFromContext(ctx, r.db)
	_, err := exec.ExecContext(
		ctx,
		q,
		file.ID,
		file.RecordID,
		file.ObjectKey,
		file.EncryptedSize,
		uploadModeToDB(file.UploadMode),
		string(file.UploadStatus),
		file.CreatedAt,
		file.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to persist record file: %w", err)
	}
	return nil
}

// UpdateUploadStatus обновляет статус загрузки файла записи пользователя.
func (r *RecordFileRepository) UpdateUploadStatus(
	ctx context.Context,
	fileID uuid.UUID,
	status model.UploadStatus,
	updatedAt time.Time,
) error {
	const q = `
UPDATE record_file
SET upload_status = $1, updated_at = $2
WHERE id = $3
`
	exec := executorFromContext(ctx, r.db)
	result, err := exec.ExecContext(ctx, q, string(status), updatedAt, fileID)
	if err != nil {
		return fmt.Errorf("failed to update record file upload status: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read affected rows count: %w", err)
	}
	if rowsAffected == 0 {
		return model.ErrRecordNotFound
	}
	return nil
}

func uploadModeToDB(uploadMode *model.UploadMode) *string {
	if uploadMode == nil {
		return nil
	}
	value := string(*uploadMode)
	return &value
}

func uploadModeFromDB(uploadMode *string) *model.UploadMode {
	if uploadMode == nil {
		return nil
	}
	value := model.UploadMode(*uploadMode)
	return &value
}
