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
	"zerogravity-82/goph-keeper/internal/usecase"
)

// RecordRepository реализует доступ к приватным записям пользователя в PostgreSQL.
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

// Create сохраняет новую приватную запись.
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

// GetByIDAndUserID возвращает приватную запись по ID записи.
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
    rf.encrypted_sha256 AS file_encrypted_sha256,
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
			return model.Record{}, usecase.ErrRecordNotFound
		}
		return model.Record{}, fmt.Errorf("failed to select record by ID and user ID: %w", err)
	}
	return recordWithFileFromDTO(row), nil
}

// ListByUserID возвращает список приватных записей.
func (r *RecordRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.RecordListItem, error) {
	const q = `
SELECT r.id, r.type, r.title, r.description, r.created_at, r.updated_at, rf.upload_status, rf.encrypted_sha256
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

// Update обновляет приватную запись и возвращает новую версию.
//
// Если приватная запись не найдена, возвращает usecase.ErrRecordNotFound.
// Если приватная запись существует, но ее версия отличается от ожидаемой, возвращает usecase.ErrRecordVersionConflict.
func (r *RecordRepository) Update(ctx context.Context, record model.Record, expectedVersion int64) (int64, error) {
	const updateQuery = `
UPDATE record
SET
    title = $1,
    description = $2,
    encrypted_dek = $3,
    encrypted_payload = $4,
    version = version + 1,
    updated_at = $5
WHERE id = $6
  AND app_user_id = $7
  AND deleted_at IS NULL
  AND version = $8
RETURNING version
`
	exec := executorFromContext(ctx, r.db)
	var version int64
	err := exec.GetContext(
		ctx,
		&version,
		updateQuery,
		record.Title,
		record.Description,
		record.EncryptedDEK.Data,
		record.EncryptedPayload.Data,
		record.UpdatedAt,
		record.ID,
		record.UserID,
		expectedVersion,
	)
	if err == nil {
		return version, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("failed to update record: %w", err)
	}

	exists, err := r.exists(ctx, record.ID, record.UserID)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, usecase.ErrRecordNotFound
	}
	return 0, usecase.ErrRecordVersionConflict
}

// ReencryptDEKs обновляет зашифрованные DEK всех активных записей пользователя при смене мастер-ключа.
//
// Метод проверяет, что клиент прислал все активные записи пользователя и что версии этих записей не изменились с
// момента чтения. При несовпадении набора или версии возвращается usecase.ErrMasterKeyChangeConflict.
func (r *RecordRepository) ReencryptDEKs(
	ctx context.Context,
	userID uuid.UUID,
	records []usecase.ReencryptedRecordDEK,
	updatedAt time.Time,
) error {
	const countQuery = `
SELECT COUNT(*)
FROM record
WHERE app_user_id = $1 AND deleted_at IS NULL
`
	exec := executorFromContext(ctx, r.db)
	var activeCount int
	if err := exec.GetContext(ctx, &activeCount, countQuery, userID); err != nil {
		return fmt.Errorf("failed to count active records: %w", err)
	}
	if activeCount != len(records) {
		return usecase.ErrMasterKeyChangeConflict
	}

	const updateQuery = `
UPDATE record
SET encrypted_dek = $1,
    version = version + 1,
    updated_at = $2
WHERE id = $3
  AND app_user_id = $4
  AND deleted_at IS NULL
  AND version = $5
`
	for _, record := range records {
		result, err := exec.ExecContext(
			ctx,
			updateQuery,
			record.EncryptedDEK,
			updatedAt,
			record.RecordID,
			userID,
			record.ExpectedVersion,
		)
		if err != nil {
			return fmt.Errorf("failed to update record encrypted DEK: %w", err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to read affected rows count: %w", err)
		}
		if rowsAffected == 0 {
			return usecase.ErrMasterKeyChangeConflict
		}
	}
	return nil
}

func (r *RecordRepository) exists(ctx context.Context, recordID uuid.UUID, userID uuid.UUID) (bool, error) {
	const q = `
SELECT EXISTS (
    SELECT 1
    FROM record
    WHERE id = $1 AND app_user_id = $2 AND deleted_at IS NULL
)
`
	exec := executorFromContext(ctx, r.db)
	var exists bool
	if err := exec.GetContext(ctx, &exists, q, recordID, userID); err != nil {
		return false, fmt.Errorf("failed to check record existence: %w", err)
	}
	return exists, nil
}

// Delete помечает приватную запись удаленной (мягкое удаление).
//
// Если приватная запись не найдена, возвращает usecase.ErrRecordNotFound.
func (r *RecordRepository) Delete(
	ctx context.Context,
	recordID uuid.UUID,
	userID uuid.UUID,
	deletedAt time.Time,
) error {
	const q = `
UPDATE record
SET deleted_at = $1, updated_at = $1
WHERE id = $2 AND app_user_id = $3 AND deleted_at IS NULL
`
	exec := executorFromContext(ctx, r.db)
	result, err := exec.ExecContext(ctx, q, deletedAt, recordID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read affected rows count: %w", err)
	}
	if rowsAffected == 0 {
		return usecase.ErrRecordNotFound
	}
	return nil
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
		item.File = &model.RecordListItemFile{
			UploadStatus:    model.UploadStatus(*row.UploadStatus),
			EncryptedSHA256: row.EncryptedSHA256,
		}
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
			ID:              *row.FileID,
			RecordID:        *row.FileRecordID,
			ObjectKey:       *row.FileObjectKey,
			EncryptedSize:   row.FileEncryptedSize,
			EncryptedSHA256: row.FileEncryptedSHA256,
			UploadStatus:    model.UploadStatus(*row.FileUploadStatus),
			CreatedAt:       *row.FileCreatedAt,
			UpdatedAt:       *row.FileUpdatedAt,
		}
	}
	return record
}

// RecordFileRepository реализует доступ к техническим данным файлов приватных записей в PostgreSQL.
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

// Create сохраняет техническую информацию о файле приватной записи.
func (r *RecordFileRepository) Create(ctx context.Context, file model.RecordFile) error {
	const q = `
INSERT INTO record_file (
    id, record_id, object_key, encrypted_size, encrypted_sha256, upload_status, created_at, updated_at
)
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
		file.EncryptedSHA256,
		string(file.UploadStatus),
		file.CreatedAt,
		file.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to persist record file: %w", err)
	}
	return nil
}

// Replace заменяет технические данные файла приватной записи при обновлении бинарной приватной записи.
func (r *RecordFileRepository) Replace(ctx context.Context, file model.RecordFile) error {
	const q = `
UPDATE record_file
SET
    id = $1,
    object_key = $2,
    encrypted_size = $3,
    encrypted_sha256 = $4,
    upload_status = $5,
    updated_at = $6
WHERE record_id = $7
`
	exec := executorFromContext(ctx, r.db)
	result, err := exec.ExecContext(
		ctx,
		q,
		file.ID,
		file.ObjectKey,
		file.EncryptedSize,
		file.EncryptedSHA256,
		string(file.UploadStatus),
		file.UpdatedAt,
		file.RecordID,
	)
	if err != nil {
		return fmt.Errorf("failed to replace record file: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read affected rows count: %w", err)
	}
	if rowsAffected == 0 {
		return usecase.ErrRecordNotFound
	}
	return nil
}

// UpdateUploadStatus обновляет статус загрузки файла приватной записи.
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
		return usecase.ErrRecordNotFound
	}
	return nil
}

// CompleteUpload переводит файл приватной записи в uploaded и сохраняет контрольную сумму зашифрованного файла.
func (r *RecordFileRepository) CompleteUpload(
	ctx context.Context,
	fileID uuid.UUID,
	encryptedSHA256 string,
	updatedAt time.Time,
) error {
	const q = `
UPDATE record_file
SET upload_status = $1, encrypted_sha256 = $2, updated_at = $3
WHERE id = $4
`
	exec := executorFromContext(ctx, r.db)
	result, err := exec.ExecContext(ctx, q, string(model.UploadStatusUploaded), encryptedSHA256, updatedAt, fileID)
	if err != nil {
		return fmt.Errorf("failed to complete record file upload: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read affected rows count: %w", err)
	}
	if rowsAffected == 0 {
		return usecase.ErrRecordNotFound
	}
	return nil
}

// MultipartUploadRepository реализует доступ к состоянию возобновляемых multipart-загрузок файлов.
type MultipartUploadRepository struct {
	db *sqlx.DB
}

// NewMultipartUploadRepository создает MultipartUploadRepository на основе подключения к БД.
func NewMultipartUploadRepository(db *sqlx.DB) (*MultipartUploadRepository, error) {
	if db == nil {
		return nil, errors.New("database connection is not provided")
	}
	return &MultipartUploadRepository{db: db}, nil
}

// Create сохраняет новую сессию multipart-загрузки.
func (r *MultipartUploadRepository) Create(ctx context.Context, upload usecase.MultipartUpload) error {
	const q = `
INSERT INTO record_file_multipart_upload (
    id, app_user_id, record_id, record_version, file_id, object_key, storage_upload_id, encrypted_size, part_size, status, created_at, updated_at, completed_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
`
	exec := executorFromContext(ctx, r.db)
	_, err := exec.ExecContext(
		ctx,
		q,
		upload.ID,
		upload.UserID,
		upload.RecordID,
		upload.RecordVersion,
		upload.FileID,
		upload.ObjectKey,
		upload.StorageUploadID,
		upload.EncryptedSize,
		upload.PartSize,
		string(upload.Status),
		upload.CreatedAt,
		upload.UpdatedAt,
		upload.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to persist multipart upload: %w", err)
	}
	return nil
}

// GetByIDAndUserID возвращает сессию multipart-загрузки по ID и пользователю.
func (r *MultipartUploadRepository) GetByIDAndUserID(
	ctx context.Context,
	uploadID uuid.UUID,
	userID uuid.UUID,
) (usecase.MultipartUpload, error) {
	const q = `
SELECT id, app_user_id, record_id, record_version, file_id, object_key, storage_upload_id, encrypted_size, part_size, status, created_at, updated_at, completed_at
FROM record_file_multipart_upload
WHERE id = $1 AND app_user_id = $2
`
	var row dto.MultipartUpload
	exec := executorFromContext(ctx, r.db)
	if err := exec.GetContext(ctx, &row, q, uploadID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return usecase.MultipartUpload{}, usecase.ErrMultipartUploadNotFound
		}
		return usecase.MultipartUpload{}, fmt.Errorf("failed to select multipart upload: %w", err)
	}
	return multipartUploadFromDTO(row), nil
}

// UpsertPart сохраняет или обновляет информацию о загруженной части.
func (r *MultipartUploadRepository) UpsertPart(ctx context.Context, part usecase.MultipartUploadPart) error {
	const q = `
INSERT INTO record_file_multipart_part (upload_id, part_number, size, etag, created_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (upload_id, part_number)
DO UPDATE SET size = EXCLUDED.size, etag = EXCLUDED.etag, created_at = EXCLUDED.created_at
`
	exec := executorFromContext(ctx, r.db)
	_, err := exec.ExecContext(ctx, q, part.UploadID, part.PartNumber, part.Size, part.ETag, part.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to upsert multipart upload part: %w", err)
	}
	return nil
}

// ListParts возвращает список загруженных частей сессии multipart-загрузки.
func (r *MultipartUploadRepository) ListParts(
	ctx context.Context,
	uploadID uuid.UUID,
) ([]usecase.MultipartUploadPart, error) {
	const q = `
SELECT upload_id, part_number, size, etag, created_at
FROM record_file_multipart_part
WHERE upload_id = $1
ORDER BY part_number
`
	var rows []dto.MultipartUploadPart
	exec := executorFromContext(ctx, r.db)
	if err := exec.SelectContext(ctx, &rows, q, uploadID); err != nil {
		return nil, fmt.Errorf("failed to select multipart upload parts: %w", err)
	}
	parts := make([]usecase.MultipartUploadPart, 0, len(rows))
	for _, row := range rows {
		parts = append(parts, multipartUploadPartFromDTO(row))
	}
	return parts, nil
}

// UpdateStatus обновляет статус сессии multipart-загрузки.
func (r *MultipartUploadRepository) UpdateStatus(
	ctx context.Context,
	uploadID uuid.UUID,
	status usecase.MultipartUploadStatus,
	updatedAt time.Time,
	completedAt *time.Time,
) error {
	const q = `
UPDATE record_file_multipart_upload
SET status = $1, updated_at = $2, completed_at = $3
WHERE id = $4
`
	exec := executorFromContext(ctx, r.db)
	result, err := exec.ExecContext(ctx, q, string(status), updatedAt, completedAt, uploadID)
	if err != nil {
		return fmt.Errorf("failed to update multipart upload status: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read affected rows count: %w", err)
	}
	if rowsAffected == 0 {
		return usecase.ErrMultipartUploadNotFound
	}
	return nil
}

func multipartUploadFromDTO(row dto.MultipartUpload) usecase.MultipartUpload {
	return usecase.MultipartUpload{
		ID:              row.ID,
		UserID:          row.UserID,
		RecordID:        row.RecordID,
		RecordVersion:   row.RecordVersion,
		FileID:          row.FileID,
		ObjectKey:       row.ObjectKey,
		StorageUploadID: row.StorageUploadID,
		EncryptedSize:   row.EncryptedSize,
		PartSize:        row.PartSize,
		Status:          usecase.MultipartUploadStatus(row.Status),
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		CompletedAt:     row.CompletedAt,
	}
}

func multipartUploadPartFromDTO(row dto.MultipartUploadPart) usecase.MultipartUploadPart {
	return usecase.MultipartUploadPart{
		UploadID:   row.UploadID,
		PartNumber: row.PartNumber,
		Size:       row.Size,
		ETag:       row.ETag,
		CreatedAt:  row.CreatedAt,
	}
}
