package model

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken описывает refresh-токен пользователя, сохраненный на сервере в виде хеша.
type RefreshToken struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	TokenHash       string
	SecurityVersion int64
	IssuedAt        time.Time
	ExpiresAt       time.Time
	RevokedAt       *time.Time
}
