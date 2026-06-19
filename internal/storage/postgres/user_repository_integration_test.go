//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/goph-keeper/internal/domain/model"
	migrationfiles "zerogravity-82/goph-keeper/migrations"
)

// TestUserRepository_CreateAndGetByLogin проверяет создание пользователя и получение пользователя по логину.
func TestUserRepository_CreateAndGetByLogin(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	repo := NewUserRepository(db)
	u := newTestUser(t, "alice")

	// Act
	err := repo.Create(ctx, u)

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
	repo := NewUserRepository(db)

	// Act
	_, err := repo.GetByLogin(ctx, "missing-user")

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, model.ErrUserNotFound))
}

// TestUserRepository_Create_DuplicateLogin проверяет маппинг нарушения уникальности логина в доменную ошибку.
func TestUserRepository_Create_DuplicateLogin(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	repo := NewUserRepository(db)
	u1 := newTestUser(t, "duplicate")
	u2 := newTestUser(t, "duplicate")

	require.NoError(t, repo.Create(ctx, u1))

	// Act
	err := repo.Create(ctx, u2)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, model.ErrLoginAlreadyTaken))
}

func openTestDB(t *testing.T, ctx context.Context) *sqlx.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URI")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URI is not set")
	}

	db, err := sqlx.ConnectContext(ctx, "pgx", dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	require.NoError(t, runTestMigrations(ctx, db))
	truncateTestTables(t, ctx, db)
	t.Cleanup(func() {
		truncateTestTables(t, ctx, db)
	})

	return db
}

func runTestMigrations(ctx context.Context, db *sqlx.DB) error {
	provider, err := goose.NewProvider(goose.DialectPostgres, db.DB, migrationfiles.FS)
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}

func truncateTestTables(t *testing.T, ctx context.Context, db *sqlx.DB) {
	t.Helper()

	_, err := db.ExecContext(ctx, "TRUNCATE TABLE app_user CASCADE")
	require.NoError(t, err)
}

func newTestUser(t *testing.T, login string) model.User {
	t.Helper()

	id, err := uuid.NewV7()
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Microsecond)

	return model.User{
		ID:            id,
		Login:         login,
		PasswordHash:  "password-hash",
		MasterKeySalt: []byte("master-key-salt"),
		RegisteredAt:  now,
		UpdatedAt:     now,
	}
}
