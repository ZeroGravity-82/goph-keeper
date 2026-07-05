// Package authcontext хранит данные аутентифицированного пользователя в context.Context.
package authcontext

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey string

const userIDContextKey ctxKey = "userID"

// WithUserID добавляет идентификатор аутентифицированного пользователя в контекст.
func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDContextKey, userID)
}

// UserIDFromContext возвращает идентификатор аутентифицированного пользователя из контекста.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	if ctx == nil {
		return uuid.Nil, false
	}
	userID, ok := ctx.Value(userIDContextKey).(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return uuid.Nil, false
	}
	return userID, true
}
