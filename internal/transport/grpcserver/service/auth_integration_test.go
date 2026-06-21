//go:build integration

package service

import (
	"context"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"zerogravity-82/goph-keeper/internal/auth"
	"zerogravity-82/goph-keeper/internal/logging"
	"zerogravity-82/goph-keeper/internal/pb"
	"zerogravity-82/goph-keeper/internal/storage/postgres"
	"zerogravity-82/goph-keeper/internal/usecase"
	migrationfiles "zerogravity-82/goph-keeper/migrations"
)

// TestAuthService_Register_Integration_OK проверяет регистрацию через gRPC-обработчик и реальные зависимости.
func TestAuthService_Register_Integration_OK(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	authService, tokenManager, userRepo, tokenRepo := newIntegrationAuthService(t, db)
	req := pb.RegisterRequest_builder{
		Login:    new("alice"),
		Password: new("password"),
	}.Build()

	// Act
	resp, err := authService.Register(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, resp.GetAccessToken())
	assert.NotEmpty(t, resp.GetRefreshToken())
	assert.Len(t, resp.GetMasterKeySalt(), 16)

	user, err := userRepo.GetByLogin(ctx, "alice")
	require.NoError(t, err)
	assert.Equal(t, resp.GetMasterKeySalt(), user.MasterKeySalt)

	claims, err := tokenManager.ParseAccessToken(resp.GetAccessToken())
	require.NoError(t, err)
	assert.Equal(t, user.ID.String(), claims.Subject)

	storedRefreshToken, err := tokenRepo.FindActiveByHash(
		ctx,
		auth.HashRefreshToken(resp.GetRefreshToken()),
		time.Now().UTC(),
	)
	require.NoError(t, err)
	assert.Equal(t, user.ID, storedRefreshToken.UserID)
	assert.NotEqual(t, resp.GetRefreshToken(), storedRefreshToken.TokenHash)
}

// TestAuthService_Register_Integration_DuplicateLogin проверяет ошибку при повторной регистрации того же логина.
func TestAuthService_Register_Integration_DuplicateLogin(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	authService, _, _, _ := newIntegrationAuthService(t, db)
	req := pb.RegisterRequest_builder{
		Login:    new("duplicate"),
		Password: new("password"),
	}.Build()

	_, err := authService.Register(ctx, req)
	require.NoError(t, err)

	// Act
	_, err = authService.Register(ctx, req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.AlreadyExists, status.Code(err))
}

func newIntegrationAuthService(
	t *testing.T,
	db *sqlx.DB,
) (*AuthService, *auth.TokenManager, *postgres.UserRepository, *postgres.RefreshTokenRepository) {
	t.Helper()

	userRepo, err := postgres.NewUserRepository(db)
	require.NoError(t, err)
	refreshTokenRepo, err := postgres.NewRefreshTokenRepository(db)
	require.NoError(t, err)
	transactor, err := postgres.NewTransactor(db)
	require.NoError(t, err)
	tokenManager, err := auth.NewTokenManager("integration-test-secret", 15*time.Minute)
	require.NoError(t, err)
	authUC, err := usecase.NewAuthUseCase(
		userRepo,
		refreshTokenRepo,
		transactor,
		tokenManager,
		30*24*time.Hour,
	)
	require.NoError(t, err)
	authService, err := NewAuthService(authUC, logging.NopLogger())
	require.NoError(t, err)

	return authService, tokenManager, userRepo, refreshTokenRepo
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
