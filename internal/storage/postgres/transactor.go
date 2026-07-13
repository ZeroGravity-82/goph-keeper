package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type ctxKey string

const txContextKey ctxKey = "tx"

type queryExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
}

// Transactor управляет транзакциями PostgreSQL.
type Transactor struct {
	db *sqlx.DB
}

// NewTransactor создает Transactor на основе подключения к БД.
func NewTransactor(db *sqlx.DB) (*Transactor, error) {
	if db == nil {
		return nil, errors.New("database connection is not provided")
	}
	return &Transactor{db: db}, nil
}

// WithinTransaction выполняет fn внутри транзакции.
//
// Если в context уже есть активная транзакция, новая транзакция не открывается, а fn выполняется в существующей.
func (t *Transactor) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if txFromContext(ctx) != nil {
		return fn(ctx)
	}

	tx, err := t.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err = fn(context.WithValue(ctx, txContextKey, tx)); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	committed = true
	return nil
}

func executorFromContext(ctx context.Context, db *sqlx.DB) queryExecutor {
	if tx := txFromContext(ctx); tx != nil {
		return tx
	}
	return db
}

func txFromContext(ctx context.Context) *sqlx.Tx {
	if ctx == nil {
		return nil
	}
	tx, _ := ctx.Value(txContextKey).(*sqlx.Tx)
	return tx
}
