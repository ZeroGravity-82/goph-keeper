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

// UserRepository реализует доступ к данным пользователя в PostgreSQL.
type UserRepository struct {
	db *sqlx.DB
}

// NewUserRepository создает UserRepository на основе подключения к БД.
func NewUserRepository(db *sqlx.DB) (*UserRepository, error) {
	if db == nil {
		return nil, errors.New("database connection is not provided")
	}
	return &UserRepository{db: db}, nil
}

// Create сохраняет нового пользователя.
func (r *UserRepository) Create(ctx context.Context, u model.User) error {
	const q = `
INSERT INTO app_user (
    id, login, password_hash, master_key_salt, master_key_verifier, security_version, registered_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
`
	exec := executorFromContext(ctx, r.db)
	_, err := exec.ExecContext(
		ctx,
		q,
		u.ID,
		u.Login,
		u.PasswordHash,
		u.MasterKeySalt,
		u.MasterKeyVerifier,
		u.SecurityVersion,
		u.RegisteredAt,
		u.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return usecase.ErrLoginAlreadyTaken
		}
		return fmt.Errorf("failed to persist new user: %w", err)
	}
	return nil
}

// GetByLogin возвращает пользователя по логину.
//
// Если пользователь не найден, возвращает usecase.ErrUserNotFound.
func (r *UserRepository) GetByLogin(ctx context.Context, login string) (model.User, error) {
	const q = `
SELECT id, login, password_hash, master_key_salt, master_key_verifier, security_version, registered_at, updated_at
FROM app_user
WHERE login = $1
`
	var u dto.User
	exec := executorFromContext(ctx, r.db)
	if err := exec.GetContext(ctx, &u, q, login); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, usecase.ErrUserNotFound
		}
		return model.User{}, fmt.Errorf("failed to select user by login: %w", err)
	}
	return model.User{
		ID:                u.ID,
		Login:             u.Login,
		PasswordHash:      u.PasswordHash,
		MasterKeySalt:     u.MasterKeySalt,
		MasterKeyVerifier: u.MasterKeyVerifier,
		SecurityVersion:   u.SecurityVersion,
		RegisteredAt:      u.RegisteredAt,
		UpdatedAt:         u.UpdatedAt,
	}, nil
}

// GetSecurityVersion возвращает текущую версию security-состояния пользователя.
func (r *UserRepository) GetSecurityVersion(ctx context.Context, userID uuid.UUID) (int64, error) {
	const q = `
SELECT security_version
FROM app_user
WHERE id = $1
`
	var securityVersion int64
	exec := executorFromContext(ctx, r.db)
	if err := exec.GetContext(ctx, &securityVersion, q, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, usecase.ErrUserNotFound
		}
		return 0, fmt.Errorf("failed to select user security version: %w", err)
	}
	return securityVersion, nil
}

// UpdateMasterKey обновляет соль и верификатор мастер-ключа пользователя и увеличивает версию security-состояния
// пользователя.
func (r *UserRepository) UpdateMasterKey(
	ctx context.Context,
	userID uuid.UUID,
	salt []byte,
	verifier []byte,
	updatedAt time.Time,
	expectedSecurityVersion int64,
) (int64, error) {
	const q = `
UPDATE app_user
SET master_key_salt = $1,
    master_key_verifier = $2,
    security_version = security_version + 1,
    updated_at = $3
WHERE id = $4
  AND security_version = $5
RETURNING security_version
`
	exec := executorFromContext(ctx, r.db)
	var securityVersion int64
	err := exec.GetContext(ctx, &securityVersion, q, salt, verifier, updatedAt, userID, expectedSecurityVersion)
	if err == nil {
		return securityVersion, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("failed to update user master key data: %w", err)
	}

	_, err = r.GetSecurityVersion(ctx, userID)
	if err != nil {
		return 0, err
	}
	return 0, usecase.ErrMasterKeyChangeConflict
}
