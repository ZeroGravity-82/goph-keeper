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
	"zerogravity-82/goph-keeper/internal/crypto"
	"zerogravity-82/goph-keeper/internal/logging"
	minioStorage "zerogravity-82/goph-keeper/internal/storage/minio"
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
func New(cfg config.ServerConfig, logger *slog.Logger) (*App, error) {
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
	tokenManager, err := auth.NewTokenManager(cfg.JWTSecret, accessTokenTTL)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to create token manager: %w", err)
	}
	userRepo, err := postgres.NewUserRepository(db)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to create user repository: %w", err)
	}
	authUC, err := buildAuthUseCase(db, tokenManager, userRepo)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	recordUC, err := buildRecordUseCase(db, cfg.FileStorage)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	grpcSrv, err := buildGRPCServer(cfg.GRPCServerAddr, tlsCert, authUC, recordUC, tokenManager, userRepo, logger)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &App{db: db, grpcSrv: grpcSrv, logger: logger}, nil
}

func buildGRPCServer(
	grpcServerAddr string,
	tlsCert tls.Certificate,
	authUC *usecase.AuthUseCase,
	recordUC *usecase.RecordUseCase,
	tokenManager *auth.TokenManager,
	sessionChecker grpcserver.UserSessionChecker,
	logger *slog.Logger,
) (*grpcserver.GRPCServer, error) {
	authService, err := buildAuthService(authUC, logger)
	if err != nil {
		return nil, err
	}
	recordsService, err := buildRecordService(recordUC, logger)
	if err != nil {
		return nil, err
	}
	grpcTlsConfig := &tls.Config{Certificates: []tls.Certificate{tlsCert}}
	grpcSrvCredentials := credentials.NewTLS(grpcTlsConfig)
	server, err := grpcserver.NewGRPCServer(
		grpcServerAddr,
		grpcSrvCredentials,
		authService,
		recordsService,
		tokenManager,
		sessionChecker,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc server: %w", err)
	}

	return server, nil
}

func buildAuthService(
	authUC *usecase.AuthUseCase,
	logger *slog.Logger,
) (*service.AuthService, error) {
	authService, err := service.NewAuthService(authUC, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth service: %w", err)
	}
	return authService, nil
}

func buildAuthUseCase(
	db *sqlx.DB,
	tokenManager *auth.TokenManager,
	userRepo *postgres.UserRepository,
) (*usecase.AuthUseCase, error) {
	recordRepo, err := postgres.NewRecordRepository(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create record repository: %w", err)
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
		recordRepo,
		refreshTokenRepo,
		transactor,
		tokenManager,
		crypto.ValidateMasterKeySalt,
		refreshTokenTTL,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth use case: %w", err)
	}
	return authUC, nil
}

func buildRecordService(recordUC *usecase.RecordUseCase, logger *slog.Logger) (*service.RecordsService, error) {
	recordsService, err := service.NewRecordsService(recordUC, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create records service: %w", err)
	}
	return recordsService, nil
}

func buildRecordUseCase(db *sqlx.DB, fileStorageCfg config.FileStorage) (*usecase.RecordUseCase, error) {
	recordRepo, err := postgres.NewRecordRepository(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create record repository: %w", err)
	}
	recordFileRepo, err := postgres.NewRecordFileRepository(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create record file repository: %w", err)
	}
	fileStorage, err := minioStorage.NewMinIOStorage(
		context.Background(),
		fileStorageCfg.Endpoint,
		fileStorageCfg.AccessKey,
		fileStorageCfg.SecretKey,
		fileStorageCfg.Bucket,
		fileStorageCfg.UseSSL,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create file storage: %w", err)
	}
	transactor, err := postgres.NewTransactor(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create transactor: %w", err)
	}
	recordUC, err := usecase.NewRecordUseCase(recordRepo, recordFileRepo, fileStorage, transactor)
	if err != nil {
		return nil, fmt.Errorf("failed to create record use case: %w", err)
	}
	return recordUC, nil
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
