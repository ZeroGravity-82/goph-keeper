package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"zerogravity-82/goph-keeper/internal/domain/model"
	"zerogravity-82/goph-keeper/internal/storage/postgres/dto"
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

// Create сохраняет нового пользователя в БД.
func (r *UserRepository) Create(ctx context.Context, u model.User) error {
	const q = `
INSERT INTO app_user (id, login, password_hash, master_key_salt, registered_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
`
	exec := executorFromContext(ctx, r.db)
	_, err := exec.ExecContext(ctx, q, u.ID, u.Login, u.PasswordHash, u.MasterKeySalt, u.RegisteredAt, u.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return model.ErrLoginAlreadyTaken
		}
		return fmt.Errorf("failed to persist new user: %w", err)
	}
	return nil
}

// GetByLogin возвращает пользователя по логину.
//
// Если пользователь не найден, возвращает model.ErrUserNotFound.
func (r *UserRepository) GetByLogin(ctx context.Context, login string) (model.User, error) {
	const q = `
SELECT id, login, password_hash, master_key_salt, registered_at, updated_at
FROM app_user
WHERE login = $1
`
	var u dto.User
	exec := executorFromContext(ctx, r.db)
	if err := exec.GetContext(ctx, &u, q, login); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, model.ErrUserNotFound
		}
		return model.User{}, fmt.Errorf("failed to select user by login: %w", err)
	}
	return model.User{
		ID:            u.ID,
		Login:         u.Login,
		PasswordHash:  u.PasswordHash,
		MasterKeySalt: u.MasterKeySalt,
		RegisteredAt:  u.RegisteredAt,
		UpdatedAt:     u.UpdatedAt,
	}, nil
}
