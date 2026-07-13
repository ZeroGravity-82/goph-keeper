package model

import (
	"time"

	"github.com/google/uuid"
)

// User описывает учетную запись пользователя.
type User struct {
	ID                uuid.UUID
	Login             string
	PasswordHash      string
	MasterKeySalt     []byte
	MasterKeyVerifier []byte
	SecurityVersion   int64
	RegisteredAt      time.Time
	UpdatedAt         time.Time
}
