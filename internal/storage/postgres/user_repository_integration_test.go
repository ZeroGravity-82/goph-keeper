//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/goph-keeper/internal/domain/model"
)

// TestUserRepository_CreateAndGetByLogin проверяет создание пользователя и получение пользователя по логину.
func TestUserRepository_CreateAndGetByLogin(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	repo, err := NewUserRepository(db)
	require.NoError(t, err)
	u := newTestUser(t, "alice")

	// Act
	err = repo.Create(ctx, u)

	// Assert
	require.NoError(t, err)

	got, err := repo.GetByLogin(ctx, u.Login)
	require.NoError(t, err)
	assert.Equal(t, u.ID, got.ID)
	assert.Equal(t, u.Login, got.Login)
	assert.Equal(t, u.PasswordHash, got.PasswordHash)
	assert.Equal(t, u.MasterKeySalt, got.MasterKeySalt)
	assert.True(t, got.RegisteredAt.Equal(u.RegisteredAt))
	assert.True(t, got.UpdatedAt.Equal(u.UpdatedAt))
}

// TestUserRepository_GetByLogin_NotFound проверяет ошибку при поиске несуществующего пользователя.
func TestUserRepository_GetByLogin_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	repo, err := NewUserRepository(db)
	require.NoError(t, err)

	// Act
	_, err = repo.GetByLogin(ctx, "missing-user")

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, model.ErrUserNotFound))
}

// TestUserRepository_Create_DuplicateLogin проверяет маппинг нарушения уникальности логина в доменную ошибку.
func TestUserRepository_Create_DuplicateLogin(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	repo, err := NewUserRepository(db)
	require.NoError(t, err)
	u1 := newTestUser(t, "duplicate")
	u2 := newTestUser(t, "duplicate")

	require.NoError(t, repo.Create(ctx, u1))

	// Act
	err = repo.Create(ctx, u2)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, model.ErrLoginAlreadyTaken))
}
