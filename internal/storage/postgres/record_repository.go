package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"zerogravity-82/goph-keeper/internal/domain/model"
)

// RecordRepository реализует доступ к приватным записям в PostgreSQL.
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

// Create сохраняет приватную запись в БД.
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

// RecordFileRepository реализует доступ к техническим данным (ключ в объектном хранилище, размер зашифрованного файла,
// режим загрузки на сервер, статус загрузки на сервер) файлов приватных записей в PostgreSQL.
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

// Create сохраняет техническую информацию о файле приватной записи в БД.
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

func uploadModeToDB(uploadMode *model.UploadMode) *string {
	if uploadMode == nil {
		return nil
	}
	value := string(*uploadMode)
	return &value
}
