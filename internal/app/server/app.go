package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/credentials"

	"zerogravity-82/goph-keeper/internal/config"
	"zerogravity-82/goph-keeper/internal/logging"
	"zerogravity-82/goph-keeper/internal/transport/grpcserver"
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
	grpcSrv := buildGRPCServer(cfg.GRPCServerAddr, tlsCert, logger)

	return &App{db: db, grpcSrv: grpcSrv, logger: logger}, nil
}

func buildGRPCServer(grpcServerAddr string, tlsCert tls.Certificate, logger *slog.Logger) *grpcserver.GRPCServer {
	grpcTlsConfig := &tls.Config{Certificates: []tls.Certificate{tlsCert}}
	grpcSrvCredentials := credentials.NewTLS(grpcTlsConfig)
	return grpcserver.NewGRPCServer(grpcServerAddr, grpcSrvCredentials, logger)
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
