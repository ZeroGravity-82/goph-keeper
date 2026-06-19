package dto

import (
	"time"

	"github.com/google/uuid"
)

// User - представление model.User в базе данных.
type User struct {
	ID            uuid.UUID `db:"id"`
	Login         string    `db:"login"`
	PasswordHash  string    `db:"password_hash"`
	MasterKeySalt []byte    `db:"master_key_salt"`
	RegisteredAt  time.Time `db:"registered_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}
