package model

import (
	"time"

	"github.com/google/uuid"
)

// RecordType представляет собой типизированный тип записи.
type RecordType string

const (
	// RecordTypeCredential обозначает запись с парой логин/пароль.
	RecordTypeCredential RecordType = "credential"
	// RecordTypeText обозначает запись с произвольными текстовыми данными.
	RecordTypeText RecordType = "text"
	// RecordTypeCard обозначает запись с данными банковской карты.
	RecordTypeCard RecordType = "card"
	// RecordTypeBinary обозначает запись с произвольными бинарными данными.
	RecordTypeBinary RecordType = "binary"
)

// EncryptedBlob содержит зашифрованные бинарные данные: сначала nonce, затем ciphertext.
// Nonce не является секретом, но должен быть уникальным для каждого шифрования с тем же ключом.
type EncryptedBlob struct {
	Data []byte
}

// Record описывает приватную запись пользователя с зашифрованными данными и открытыми метаданными.
type Record struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	Type             RecordType
	Title            string
	Description      string
	EncryptedDEK     EncryptedBlob
	EncryptedPayload EncryptedBlob
	Version          int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
	File             *RecordFile
}

// RecordListItem описывает краткое представление приватной записи для списка записей пользователя.
type RecordListItem struct {
	ID          uuid.UUID
	Type        RecordType
	Title       string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	File        *RecordListItemFile
}

const (
	// UploadModeSinglePart обозначает загрузку файла одним объектом.
	UploadModeSinglePart UploadMode = "single_part"
	// UploadModeMultiPart обозначает загрузку файла несколькими частями.
	UploadModeMultiPart UploadMode = "multipart"
)

// UploadMode представляет собой типизированный режим загрузки файла на сервер.
type UploadMode string

const (
	// UploadStatusPending обозначает, что загрузка файла на сервер еще не началась.
	UploadStatusPending UploadStatus = "pending"
	// UploadStatusUploading обозначает, что загрузка файла на сервер находится в процессе.
	UploadStatusUploading UploadStatus = "uploading"
	// UploadStatusUploaded обозначает, что загрузка файла на сервер завершилась успешно.
	UploadStatusUploaded UploadStatus = "uploaded"
	// UploadStatusFailed обозначает, что загрузка файла на сервер завершилась неудачей.
	UploadStatusFailed UploadStatus = "failed"
)

// UploadStatus представляет собой типизированный статус загрузки файла на сервер.
type UploadStatus string

// RecordFile описывает техническую информацию о зашифрованном файле в объектном хранилище.
type RecordFile struct {
	ID            uuid.UUID
	RecordID      uuid.UUID
	ObjectKey     string
	EncryptedSize *int64
	UploadMode    *UploadMode
	UploadStatus  UploadStatus
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// RecordListItemFile описывает краткое представление файла приватной записи для списка записей пользователя.
type RecordListItemFile struct {
	UploadStatus UploadStatus
}
