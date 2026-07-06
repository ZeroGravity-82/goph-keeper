// Package authcontext хранит данные аутентифицированного пользователя в context.Context.
package authcontext

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey string

const (
	userIDContextKey          ctxKey = "userID"
	securityVersionContextKey ctxKey = "securityVersion"
)

// WithUserID добавляет идентификатор аутентифицированного пользователя в контекст.
func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDContextKey, userID)
}

// WithUserSession добавляет идентификатор пользователя и версию security-состояния пользователя в контекст.
func WithUserSession(ctx context.Context, userID uuid.UUID, securityVersion int64) context.Context {
	ctx = WithUserID(ctx, userID)
	return context.WithValue(ctx, securityVersionContextKey, securityVersion)
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

// SecurityVersionFromContext возвращает версию security-состояния пользователя из контекста.
func SecurityVersionFromContext(ctx context.Context) (int64, bool) {
	if ctx == nil {
		return 0, false
	}
	securityVersion, ok := ctx.Value(securityVersionContextKey).(int64)
	if !ok || securityVersion <= 0 {
		return 0, false
	}
	return securityVersion, true
}
