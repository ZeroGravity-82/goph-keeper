package dto

import (
	"time"

	"github.com/google/uuid"
)

// Record описывает строку таблицы record, представляющую model.Record в базе данных.
type Record struct {
	ID               uuid.UUID  `db:"id"`
	UserID           uuid.UUID  `db:"app_user_id"`
	Type             string     `db:"type"`
	Title            string     `db:"title"`
	Description      string     `db:"description"`
	EncryptedDEK     []byte     `db:"encrypted_dek"`
	EncryptedPayload []byte     `db:"encrypted_payload"`
	Version          int64      `db:"version"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
	DeletedAt        *time.Time `db:"deleted_at"`
}

// RecordWithFile описывает строку выборки приватной записи с опциональной связанной строкой record_file.
type RecordWithFile struct {
	ID               uuid.UUID  `db:"id"`
	UserID           uuid.UUID  `db:"app_user_id"`
	Type             string     `db:"type"`
	Title            string     `db:"title"`
	Description      string     `db:"description"`
	EncryptedDEK     []byte     `db:"encrypted_dek"`
	EncryptedPayload []byte     `db:"encrypted_payload"`
	Version          int64      `db:"version"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
	DeletedAt        *time.Time `db:"deleted_at"`

	FileID            *uuid.UUID `db:"file_id"`
	FileRecordID      *uuid.UUID `db:"file_record_id"`
	FileObjectKey     *string    `db:"file_object_key"`
	FileEncryptedSize *int64     `db:"file_encrypted_size"`
	FileUploadMode    *string    `db:"file_upload_mode"`
	FileUploadStatus  *string    `db:"file_upload_status"`
	FileCreatedAt     *time.Time `db:"file_created_at"`
	FileUpdatedAt     *time.Time `db:"file_updated_at"`
}

// RecordListItem описывает строку выборки списка приватных записей из базы данных.
type RecordListItem struct {
	ID           uuid.UUID `db:"id"`
	Type         string    `db:"type"`
	Title        string    `db:"title"`
	Description  string    `db:"description"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
	UploadStatus *string   `db:"upload_status"`
}

// RecordFile описывает строку таблицы record_file, представляющую model.RecordFile в базе данных.
type RecordFile struct {
	ID            uuid.UUID `db:"id"`
	RecordID      uuid.UUID `db:"record_id"`
	ObjectKey     string    `db:"object_key"`
	EncryptedSize *int64    `db:"encrypted_size"`
	UploadMode    *string   `db:"upload_mode"`
	UploadStatus  string    `db:"upload_status"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}
