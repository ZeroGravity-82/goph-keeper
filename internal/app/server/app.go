package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/credentials"

	"zerogravity-82/goph-keeper/internal/auth"
	"zerogravity-82/goph-keeper/internal/config"
	"zerogravity-82/goph-keeper/internal/logging"
	"zerogravity-82/goph-keeper/internal/storage/postgres"
	"zerogravity-82/goph-keeper/internal/transport/grpcserver"
	"zerogravity-82/goph-keeper/internal/transport/grpcserver/service"
	"zerogravity-82/goph-keeper/internal/usecase"
)

const (
	accessTokenTTL  = 15 * time.Minute // TODO вынести TTL в конфиг
	refreshTokenTTL = 30 * 24 * time.Hour
)

// App инициализирует зависимости сервиса.
type App struct {
	db      *sqlx.DB
	grpcSrv *grpcserver.GRPCServer
	logger  *slog.Logger
}

// New создает App - подключается к БД и настраивает прикладные сервисы.
func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	if logger == nil {
		logger = logging.NopLogger()
	}

	db, err := sqlx.Connect("pgx", cfg.DatabaseURI)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to the database: %w", err)
	}

	tlsCert, err := tls.LoadX509KeyPair(cfg.TLSCertPath, cfg.TLSKeyPath)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to load tls certificate: %w", err)
	}
	authUC, err := buildAuthUseCase(db, cfg.JWTSecret)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	authService, err := service.NewAuthService(authUC, logger)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to create auth service: %w", err)
	}
	grpcSrv, err := buildGRPCServer(cfg.GRPCServerAddr, tlsCert, authService, logger)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &App{db: db, grpcSrv: grpcSrv, logger: logger}, nil
}

func buildAuthUseCase(db *sqlx.DB, jwtSecret string) (*usecase.AuthUseCase, error) {
	jwtManager, err := auth.NewJWTManager(jwtSecret, accessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWT manager: %w", err)
	}
	userRepo, err := postgres.NewUserRepository(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create user repository: %w", err)
	}
	refreshTokenRepo, err := postgres.NewRefreshTokenRepository(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh token repository: %w", err)
	}
	transactor, err := postgres.NewTransactor(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create transactor: %w", err)
	}
	authUC, err := usecase.NewAuthUseCase(
		userRepo,
		refreshTokenRepo,
		transactor,
		jwtManager,
		refreshTokenTTL,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth use case: %w", err)
	}
	return authUC, nil
}

func buildGRPCServer(
	grpcServerAddr string,
	tlsCert tls.Certificate,
	authService *service.AuthService,
	logger *slog.Logger,
) (*grpcserver.GRPCServer, error) {
	grpcTlsConfig := &tls.Config{Certificates: []tls.Certificate{tlsCert}}
	grpcSrvCredentials := credentials.NewTLS(grpcTlsConfig)
	server, err := grpcserver.NewGRPCServer(grpcServerAddr, grpcSrvCredentials, authService, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc server: %w", err)
	}

	return server, nil
}

// Run применяет миграции БД и запускает gRPC-сервер.
// Блокируется до остановки по сигналу завершения или из-за ошибки gRPC-сервера.
func (a *App) Run(ctx context.Context) error {
	err := a.runMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error { return a.grpcSrv.Run(ctx) })
	return eg.Wait()
}

// Close закрывает ресурсы приложения (например, соединение с БД).
func (a *App) Close() error {
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}
