package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// RecordMutationGuard сериализует изменяющие операции с приватными записями одного пользователя в PostgreSQL.
type RecordMutationGuard struct {
	db         *sqlx.DB
	transactor *Transactor
}

// NewRecordMutationGuard создает RecordMutationGuard на основе подключения к БД.
func NewRecordMutationGuard(db *sqlx.DB) (*RecordMutationGuard, error) {
	if db == nil {
		return nil, errors.New("database connection is not provided")
	}
	transactor, err := NewTransactor(db)
	if err != nil {
		return nil, err
	}
	return &RecordMutationGuard{db: db, transactor: transactor}, nil
}

// WithUserRecordsLock выполняет функцию fn в транзакции с Advisory Lock по идентификатору пользователя.
func (g *RecordMutationGuard) WithUserRecordsLock(
	ctx context.Context,
	userID uuid.UUID,
	fn func(ctx context.Context) error,
) error {
	return g.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		const q = `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))`
		exec := executorFromContext(ctx, g.db)
		if _, err := exec.ExecContext(ctx, q, userID); err != nil {
			return fmt.Errorf("failed to lock user records: %w", err)
		}
		return fn(ctx)
	})
}
