//go:build integration

package service

import (
	"context"
	"errors"
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
	"zerogravity-82/goph-keeper/internal/crypto"
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
	req := registerRequest("alice", "password")

	// Act
	resp, err := authService.Register(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, resp.GetAccessToken())
	assert.NotEmpty(t, resp.GetRefreshToken())
	assert.Len(t, resp.GetMasterKeySalt(), 16)

	user, err := userRepo.GetByLogin(ctx, "alice")
	require.NoError(t, err)
	assert.Equal(t, user.MasterKeySalt, resp.GetMasterKeySalt())

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
}

// TestAuthService_Register_Integration_DuplicateLogin проверяет ошибку при повторной регистрации того же логина.
func TestAuthService_Register_Integration_DuplicateLogin(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	authService, _, _, _ := newIntegrationAuthService(t, db)
	req := registerRequest("duplicate", "password")

	_, err := authService.Register(ctx, req)
	require.NoError(t, err)

	// Act
	_, err = authService.Register(ctx, req)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.AlreadyExists, status.Code(err))
}

// TestAuthService_Login_Integration_OK проверяет аутентификацию через gRPC-обработчик и реальные зависимости.
func TestAuthService_Login_Integration_OK(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	authService, tokenManager, userRepo, tokenRepo := newIntegrationAuthService(t, db)

	registerReq := registerRequest("login-user", "password")
	registerResp, err := authService.Register(ctx, registerReq)
	require.NoError(t, err)

	loginReq := pb.LoginRequest_builder{
		Login:    new("login-user"),
		Password: new("password"),
	}.Build()

	// Act
	loginResp, err := authService.Login(ctx, loginReq)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, loginResp.GetAccessToken())
	assert.NotEmpty(t, loginResp.GetRefreshToken())
	assert.NotEqual(t, registerResp.GetRefreshToken(), loginResp.GetRefreshToken())
	assert.Equal(t, registerResp.GetMasterKeySalt(), loginResp.GetMasterKeySalt())

	user, err := userRepo.GetByLogin(ctx, "login-user")
	require.NoError(t, err)
	assert.Equal(t, user.MasterKeySalt, loginResp.GetMasterKeySalt())
	assert.Equal(t, user.MasterKeyVerifier, loginResp.GetMasterKeyVerifier())

	claims, err := tokenManager.ParseAccessToken(loginResp.GetAccessToken())
	require.NoError(t, err)
	assert.Equal(t, user.ID.String(), claims.Subject)

	storedRefreshToken, err := tokenRepo.FindActiveByHash(
		ctx,
		auth.HashRefreshToken(loginResp.GetRefreshToken()),
		time.Now().UTC(),
	)
	require.NoError(t, err)
	assert.Equal(t, user.ID, storedRefreshToken.UserID)
}

// TestAuthService_Login_Integration_WrongPassword проверяет ошибку при неверном пароле.
func TestAuthService_Login_Integration_WrongPassword(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	authService, _, _, _ := newIntegrationAuthService(t, db)

	registerReq := registerRequest("login-user", "password")
	_, err := authService.Register(ctx, registerReq)
	require.NoError(t, err)

	loginReq := pb.LoginRequest_builder{
		Login:    new("login-user"),
		Password: new("another-password"),
	}.Build()

	// Act
	_, err = authService.Login(ctx, loginReq)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestAuthService_Login_Integration_UnknownUser проверяет ошибку при неизвестном логине.
func TestAuthService_Login_Integration_UnknownUser(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	authService, _, _, _ := newIntegrationAuthService(t, db)
	loginReq := pb.LoginRequest_builder{
		Login:    new("unknown"),
		Password: new("password"),
	}.Build()

	// Act
	_, err := authService.Login(ctx, loginReq)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestAuthService_Refresh_Integration_OK проверяет ротацию refresh-токена через gRPC-обработчик и реальные зависимости.
func TestAuthService_Refresh_Integration_OK(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	authService, tokenManager, userRepo, tokenRepo := newIntegrationAuthService(t, db)

	registerReq := registerRequest("refresh-user", "password")
	registerResp, err := authService.Register(ctx, registerReq)
	require.NoError(t, err)

	refreshReq := pb.RefreshRequest_builder{RefreshToken: new(registerResp.GetRefreshToken())}.Build()

	// Act
	refreshResp, err := authService.Refresh(ctx, refreshReq)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, refreshResp.GetAccessToken())
	assert.NotEmpty(t, refreshResp.GetRefreshToken())
	assert.NotEqual(t, registerResp.GetRefreshToken(), refreshResp.GetRefreshToken())

	user, err := userRepo.GetByLogin(ctx, "refresh-user")
	require.NoError(t, err)

	claims, err := tokenManager.ParseAccessToken(refreshResp.GetAccessToken())
	require.NoError(t, err)
	assert.Equal(t, user.ID.String(), claims.Subject)

	_, err = tokenRepo.FindActiveByHash(
		ctx,
		auth.HashRefreshToken(registerResp.GetRefreshToken()),
		time.Now().UTC(),
	)
	require.Error(t, err)
	assert.True(t, errors.Is(err, usecase.ErrRefreshTokenNotFound))

	storedRefreshToken, err := tokenRepo.FindActiveByHash(
		ctx,
		auth.HashRefreshToken(refreshResp.GetRefreshToken()),
		time.Now().UTC(),
	)
	require.NoError(t, err)
	assert.Equal(t, user.ID, storedRefreshToken.UserID)
}

// TestAuthService_Refresh_Integration_ReusedToken проверяет ошибку при повторном использовании refresh-токена.
func TestAuthService_Refresh_Integration_ReusedToken(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	authService, _, _, _ := newIntegrationAuthService(t, db)

	registerReq := registerRequest("reused-refresh", "password")
	registerResp, err := authService.Register(ctx, registerReq)
	require.NoError(t, err)

	refreshReq := pb.RefreshRequest_builder{RefreshToken: new(registerResp.GetRefreshToken())}.Build()
	_, err = authService.Refresh(ctx, refreshReq)
	require.NoError(t, err)

	// Act
	_, err = authService.Refresh(ctx, refreshReq)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestAuthService_Refresh_Integration_UnknownToken проверяет ошибку при неизвестном refresh-токене.
func TestAuthService_Refresh_Integration_UnknownToken(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	authService, _, _, _ := newIntegrationAuthService(t, db)
	refreshReq := pb.RefreshRequest_builder{RefreshToken: new("unknown-refresh-token")}.Build()

	// Act
	_, err := authService.Refresh(ctx, refreshReq)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestAuthService_Logout_Integration_OK проверяет отзыв refresh-токена через gRPC-обработчик и реальные зависимости.
func TestAuthService_Logout_Integration_OK(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	authService, _, _, tokenRepo := newIntegrationAuthService(t, db)

	registerReq := registerRequest("logout-user", "password")
	registerResp, err := authService.Register(ctx, registerReq)
	require.NoError(t, err)

	logoutReq := pb.LogoutRequest_builder{RefreshToken: new(registerResp.GetRefreshToken())}.Build()

	// Act
	resp, err := authService.Logout(ctx, logoutReq)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp)

	_, err = tokenRepo.FindActiveByHash(
		ctx,
		auth.HashRefreshToken(registerResp.GetRefreshToken()),
		time.Now().UTC(),
	)
	require.Error(t, err)
	assert.True(t, errors.Is(err, usecase.ErrRefreshTokenNotFound))
}

// TestAuthService_Logout_Integration_ReusedToken проверяет ошибку при повторном завершении той же сессии.
func TestAuthService_Logout_Integration_ReusedToken(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	authService, _, _, _ := newIntegrationAuthService(t, db)

	registerReq := registerRequest("reused-logout", "password")
	registerResp, err := authService.Register(ctx, registerReq)
	require.NoError(t, err)

	logoutReq := pb.LogoutRequest_builder{RefreshToken: new(registerResp.GetRefreshToken())}.Build()
	_, err = authService.Logout(ctx, logoutReq)
	require.NoError(t, err)

	// Act
	_, err = authService.Logout(ctx, logoutReq)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestAuthService_Logout_Integration_UnknownToken проверяет ошибку при неизвестном refresh-токене.
func TestAuthService_Logout_Integration_UnknownToken(t *testing.T) {
	// Arrange
	ctx := context.Background()
	db := openTestDB(t, ctx)
	authService, _, _, _ := newIntegrationAuthService(t, db)
	logoutReq := pb.LogoutRequest_builder{RefreshToken: new("unknown-refresh-token")}.Build()

	// Act
	_, err := authService.Logout(ctx, logoutReq)

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
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
		crypto.ValidateMasterKeySalt,
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
