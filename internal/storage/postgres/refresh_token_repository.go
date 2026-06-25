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

// RefreshTokenRepository реализует доступ к данным refresh-токенов в PostgreSQL.
type RefreshTokenRepository struct {
	db *sqlx.DB
}

// NewRefreshTokenRepository создает RefreshTokenRepository на основе подключения к БД.
func NewRefreshTokenRepository(db *sqlx.DB) (*RefreshTokenRepository, error) {
	if db == nil {
		return nil, errors.New("database connection is not provided")
	}
	return &RefreshTokenRepository{db: db}, nil
}

// Create сохраняет refresh-токен в БД.
func (r *RefreshTokenRepository) Create(ctx context.Context, token model.RefreshToken) error {
	const q = `
INSERT INTO refresh_token (id, app_user_id, token_hash, issued_at, expires_at, revoked_at)
VALUES ($1, $2, $3, $4, $5, $6)
`
	exec := executorFromContext(ctx, r.db)
	_, err := exec.ExecContext(
		ctx,
		q,
		token.ID,
		token.UserID,
		token.TokenHash,
		token.IssuedAt,
		token.ExpiresAt,
		token.RevokedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to persist refresh token: %w", err)
	}
	return nil
}

// FindActiveByHash возвращает активный refresh-токен по хешу и блокирует найденную строку до конца транзакции.
//
// Активным считается токен, который не отозван и срок действия которого еще не истек.
// Если активный токен не найден, возвращает usecase.ErrRefreshTokenNotFound.
func (r *RefreshTokenRepository) FindActiveByHash(
	ctx context.Context,
	tokenHash string,
	now time.Time,
) (model.RefreshToken, error) {
	const q = `
SELECT id, app_user_id, token_hash, issued_at, expires_at, revoked_at
FROM refresh_token
WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > $2
FOR UPDATE
`
	var token dto.RefreshToken
	exec := executorFromContext(ctx, r.db)
	if err := exec.GetContext(ctx, &token, q, tokenHash, now); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.RefreshToken{}, usecase.ErrRefreshTokenNotFound
		}
		return model.RefreshToken{}, fmt.Errorf("failed to select active refresh token by hash: %w", err)
	}
	return refreshTokenToModel(token), nil
}

// Revoke отзывает refresh-токен.
//
// Если токен не найден, возвращает usecase.ErrRefreshTokenNotFound.
func (r *RefreshTokenRepository) Revoke(ctx context.Context, tokenID uuid.UUID, revokedAt time.Time) error {
	const q = `
UPDATE refresh_token
SET revoked_at = $2
WHERE id = $1
`
	exec := executorFromContext(ctx, r.db)
	res, err := exec.ExecContext(ctx, q, tokenID, revokedAt)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get revoked refresh token count: %w", err)
	}
	if affected == 0 {
		return usecase.ErrRefreshTokenNotFound
	}
	return nil
}

func refreshTokenToModel(token dto.RefreshToken) model.RefreshToken {
	return model.RefreshToken{
		ID:        token.ID,
		UserID:    token.UserID,
		TokenHash: token.TokenHash,
		IssuedAt:  token.IssuedAt,
		ExpiresAt: token.ExpiresAt,
		RevokedAt: token.RevokedAt,
	}
}
