package usecase

import (
	"context"

	"github.com/google/uuid"
)

// recordMutationGuard сериализует изменяющие операции с приватными записями одного пользователя.
type recordMutationGuard interface {
	WithUserRecordsLock(ctx context.Context, userID uuid.UUID, fn func(ctx context.Context) error) error
}
