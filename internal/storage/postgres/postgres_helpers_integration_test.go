//go:build integration

package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"

	"zerogravity-82/goph-keeper/internal/domain/model"
	migrationfiles "zerogravity-82/goph-keeper/migrations"
)

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
	now := fixedTestTime()

	return model.User{
		ID:                id,
		Login:             login,
		PasswordHash:      "password-hash",
		MasterKeySalt:     []byte("1234567890abcdef"),
		MasterKeyVerifier: []byte("master-key-verifier"),
		RegisteredAt:      now,
		UpdatedAt:         now,
	}
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
