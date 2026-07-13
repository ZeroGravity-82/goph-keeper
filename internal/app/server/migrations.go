package server

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	migrationfiles "zerogravity-82/goph-keeper/migrations"
)

// runMigrations накатывает встроенные goose-миграции при старте сервиса.
//
// Для защиты от конкурентного запуска используется PostgreSQL advisory lock через session locker пакета goose:
// несколько инстансов сервиса будут ждать одну и ту же блокировку, поэтому миграции в конкретный момент времени накатит
// только один из инстансов.
//
// Реализация session locker в goose: https://github.com/pressly/goose/blob/main/lock/postgres.go
// Документация PostgreSQL по advisory lock: https://www.postgresql.org/docs/current/explicit-locking.html#ADVISORY-LOCKS
func (a *App) runMigrations(ctx context.Context) error {
	const (
		migrationLockRetryPeriodSeconds  = 5
		migrationLockRetryAttempts       = 60
		migrationUnlockRetryPeriodSecond = 2
		migrationUnlockRetryAttempts     = 30
	)

	sessionLocker, err := lock.NewPostgresSessionLocker(
		lock.WithLockID(lock.DefaultLockID),
		lock.WithLockTimeout(migrationLockRetryPeriodSeconds, migrationLockRetryAttempts),
		lock.WithUnlockTimeout(migrationUnlockRetryPeriodSecond, migrationUnlockRetryAttempts),
	)
	if err != nil {
		return fmt.Errorf("failed to create postgres session locker: %w", err)
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		a.db.DB,
		migrationfiles.FS,
		goose.WithSessionLocker(sessionLocker),
	)
	if err != nil {
		return fmt.Errorf("failed to create goose provider: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	for _, res := range results {
		a.logger.Info("applied migration",
			slog.Int64("version", res.Source.Version),
			slog.String("source", res.Source.Path),
			slog.Duration("duration", res.Duration),
		)
	}
	a.logger.Info("postgres migrations complete", slog.Int("count", len(results)))

	return nil
}
