package dto

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken описывает строку таблицы refresh_token, представляющую model.RefreshToken в базе данных.
type RefreshToken struct {
	ID        uuid.UUID  `db:"id"`
	UserID    uuid.UUID  `db:"app_user_id"`
	TokenHash string     `db:"token_hash"`
	IssuedAt  time.Time  `db:"issued_at"`
	ExpiresAt time.Time  `db:"expires_at"`
	RevokedAt *time.Time `db:"revoked_at"`
}
