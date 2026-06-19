package server

import (
	"context"
	"fmt"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"zerogravity-82/goph-keeper/internal/config"
	"zerogravity-82/goph-keeper/internal/logging"
)

// App инициализирует зависимости сервиса.
type App struct {
	db     *sqlx.DB
	logger *slog.Logger
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

	return &App{
		db:     db,
		logger: logger,
	}, nil
}

// Run применяет миграции БД и запускает gRPC-сервер.
// Блокируется до остановки по сигналу завершения или из-за ошибки gRPC-сервера.
func (a *App) Run(ctx context.Context) error {
	ctx = logging.WithLogger(ctx, a.logger)

	err := a.runMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	//eg, ctx := errgroup.WithContext(ctx)
	//eg.Go(func() error { return a.srv.Run(ctx) })
	//return eg.Wait()

	return nil
}

// Close закрывает ресурсы приложения (например, соединение с БД).
func (a *App) Close() {
	if a.db != nil {
		if err := a.db.Close(); err != nil {
			a.logger.Error("failed to close db", slog.Any("err", err))
		}
	}
}
