//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/goph-keeper/internal/domain/model"
)

// TestTransactor_WithinTransaction_Commit проверяет фиксацию изменений после успешного выполнения callback.
func TestTransactor_WithinTransaction_Commit(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	tr, err := NewTransactor(db)
	require.NoError(t, err)
	userRepo, err := NewUserRepository(db)
	require.NoError(t, err)
	tokenRepo, err := NewRefreshTokenRepository(db)
	require.NoError(t, err)
	user := newTestUser(t, "tx-commit-user")
	token := newTestRefreshToken(t, user.ID, "tx-commit-token", time.Now().UTC())

	// Act
	err = tr.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := userRepo.Create(ctx, user); err != nil {
			return err
		}
		if err := tokenRepo.Create(ctx, token); err != nil {
			return err
		}
		return nil
	})

	// Assert
	require.NoError(t, err)

	gotUser, err := userRepo.GetByLogin(ctx, user.Login)
	require.NoError(t, err)
	assert.Equal(t, user.ID, gotUser.ID)

	gotToken, err := tokenRepo.FindActiveByHash(ctx, token.TokenHash, token.IssuedAt)
	require.NoError(t, err)
	assert.Equal(t, token.ID, gotToken.ID)
}

// TestTransactor_WithinTransaction_Rollback проверяет откат изменений, если callback вернул ошибку.
func TestTransactor_WithinTransaction_Rollback(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	tr, err := NewTransactor(db)
	require.NoError(t, err)
	userRepo, err := NewUserRepository(db)
	require.NoError(t, err)
	tokenRepo, err := NewRefreshTokenRepository(db)
	require.NoError(t, err)
	user := newTestUser(t, "tx-rollback-user")
	token := newTestRefreshToken(t, user.ID, "tx-rollback-token", time.Now().UTC())
	wantErr := errors.New("fail transaction")

	// Act
	err = tr.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := userRepo.Create(ctx, user); err != nil {
			return err
		}
		if err := tokenRepo.Create(ctx, token); err != nil {
			return err
		}
		return wantErr
	})

	// Assert
	require.ErrorIs(t, err, wantErr)

	_, err = userRepo.GetByLogin(ctx, user.Login)
	require.Error(t, err)
	assert.True(t, errors.Is(err, model.ErrUserNotFound))

	_, err = tokenRepo.FindActiveByHash(ctx, token.TokenHash, token.IssuedAt)
	require.Error(t, err)
	assert.True(t, errors.Is(err, model.ErrRefreshTokenNotFound))
}
