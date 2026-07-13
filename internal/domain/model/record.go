package model

import (
	"time"

	"github.com/google/uuid"
)

// RecordType представляет собой типизированный тип приватной записи.
type RecordType string

const (
	// RecordTypeCredential обозначает приватную запись с парой логин/пароль.
	RecordTypeCredential RecordType = "credential"
	// RecordTypeText обозначает приватную запись с произвольными текстовыми данными.
	RecordTypeText RecordType = "text"
	// RecordTypeCard обозначает приватную запись с данными банковской карты.
	RecordTypeCard RecordType = "card"
	// RecordTypeBinary обозначает приватную запись с произвольными бинарными данными.
	RecordTypeBinary RecordType = "binary"
)

// EncryptedBlob содержит зашифрованные бинарные данные: сначала nonce, затем ciphertext.
// Nonce не является секретом, но должен быть уникальным для каждого шифрования с тем же ключом.
type EncryptedBlob struct {
	Data []byte
}

// Record описывает приватную запись с зашифрованными данными и открытыми метаданными.
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

// RecordListItem описывает краткое представление приватной записи для списка.
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
	ID              uuid.UUID
	RecordID        uuid.UUID
	ObjectKey       string
	EncryptedSize   *int64
	EncryptedSHA256 *string
	UploadStatus    UploadStatus
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// RecordListItemFile описывает краткое представление файла приватной записи для списка.
type RecordListItemFile struct {
	UploadStatus    UploadStatus
	EncryptedSHA256 *string
}
