//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/goph-keeper/internal/domain/model"
)

// TestRefreshTokenRepository_CreateAndFindActiveByHash проверяет создание и получение активного refresh-токена.
func TestRefreshTokenRepository_CreateAndFindActiveByHash(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	userRepo, err := NewUserRepository(db)
	require.NoError(t, err)
	tokenRepo, err := NewRefreshTokenRepository(db)
	require.NoError(t, err)
	user := newTestUser(t, "refresh-user")
	token := newTestRefreshToken(t, user.ID, "active-token-hash", time.Now().UTC())

	require.NoError(t, userRepo.Create(ctx, user))

	// Act
	err = tokenRepo.Create(ctx, token)

	// Assert
	require.NoError(t, err)

	got, err := tokenRepo.FindActiveByHash(ctx, token.TokenHash, token.IssuedAt)
	require.NoError(t, err)
	assert.Equal(t, token.ID, got.ID)
	assert.Equal(t, token.UserID, got.UserID)
	assert.Equal(t, token.TokenHash, got.TokenHash)
	assert.True(t, got.IssuedAt.Equal(token.IssuedAt))
	assert.True(t, got.ExpiresAt.Equal(token.ExpiresAt))
	assert.Nil(t, got.RevokedAt)
}

// TestRefreshTokenRepository_FindActiveByHash_NotFound проверяет ошибку при поиске отсутствующего refresh-токена.
func TestRefreshTokenRepository_FindActiveByHash_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	tokenRepo, err := NewRefreshTokenRepository(db)
	require.NoError(t, err)

	// Act
	_, err = tokenRepo.FindActiveByHash(ctx, "missing-token-hash", time.Now().UTC())

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, model.ErrRefreshTokenNotFound))
}

// TestRefreshTokenRepository_FindActiveByHash_Expired проверяет, что истекший refresh-токен не считается активным.
func TestRefreshTokenRepository_FindActiveByHash_Expired(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	userRepo, err := NewUserRepository(db)
	require.NoError(t, err)
	tokenRepo, err := NewRefreshTokenRepository(db)
	require.NoError(t, err)
	user := newTestUser(t, "expired-token-user")
	now := time.Now().UTC()
	token := newTestRefreshToken(t, user.ID, "expired-token-hash", now.Add(-2*time.Hour))
	token.ExpiresAt = now.Add(-time.Hour)

	require.NoError(t, userRepo.Create(ctx, user))
	require.NoError(t, tokenRepo.Create(ctx, token))

	// Act
	_, err = tokenRepo.FindActiveByHash(ctx, token.TokenHash, now)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, model.ErrRefreshTokenNotFound))
}

// TestRefreshTokenRepository_Revoke проверяет отзыв refresh-токена.
func TestRefreshTokenRepository_Revoke(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	userRepo, err := NewUserRepository(db)
	require.NoError(t, err)
	tokenRepo, err := NewRefreshTokenRepository(db)
	require.NoError(t, err)
	user := newTestUser(t, "revoked-token-user")
	now := time.Now().UTC()
	token := newTestRefreshToken(t, user.ID, "revoked-token-hash", now)
	revokedAt := now.Add(time.Minute)

	require.NoError(t, userRepo.Create(ctx, user))
	require.NoError(t, tokenRepo.Create(ctx, token))

	// Act
	err = tokenRepo.Revoke(ctx, token.ID, revokedAt)

	// Assert
	require.NoError(t, err)
	_, err = tokenRepo.FindActiveByHash(ctx, token.TokenHash, revokedAt)
	require.Error(t, err)
	assert.True(t, errors.Is(err, model.ErrRefreshTokenNotFound))
}

// TestRefreshTokenRepository_Revoke_NotFound проверяет ошибку при отзыве отсутствующего refresh-токена.
func TestRefreshTokenRepository_Revoke_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	tokenRepo, err := NewRefreshTokenRepository(db)
	require.NoError(t, err)

	// Act
	err = tokenRepo.Revoke(ctx, uuid.New(), time.Now().UTC())

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, model.ErrRefreshTokenNotFound))
}

func newTestRefreshToken(t *testing.T, userID uuid.UUID, tokenHash string, issuedAt time.Time) model.RefreshToken {
	t.Helper()

	id, err := uuid.NewV7()
	require.NoError(t, err)
	issuedAt = issuedAt.UTC().Truncate(time.Microsecond)

	return model.RefreshToken{
		ID:        id,
		UserID:    userID,
		TokenHash: tokenHash,
		IssuedAt:  issuedAt,
		ExpiresAt: issuedAt.Add(time.Hour),
		RevokedAt: nil,
	}
}
