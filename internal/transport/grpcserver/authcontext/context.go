// Package authcontext хранит данные аутентифицированного пользователя в context.Context.
package authcontext

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type ctxKey string

const (
	userIDContextKey          ctxKey = "userID"
	securityVersionContextKey ctxKey = "securityVersion"
)

var (
	errUserIDMissing               = errors.New("user ID is missing")
	errUserIDInvalidType           = errors.New("user ID has invalid type")
	errUserIDInvalidValue          = errors.New("user ID is invalid")
	errSecurityVersionMissing      = errors.New("security version is missing")
	errSecurityVersionInvalidType  = errors.New("security version has invalid type")
	errSecurityVersionInvalidValue = errors.New("security version is invalid")
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
func UserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	if ctx == nil {
		return uuid.Nil, errUserIDMissing
	}
	value := ctx.Value(userIDContextKey)
	if value == nil {
		return uuid.Nil, errUserIDMissing
	}

	userID, ok := value.(uuid.UUID)
	if !ok {
		return uuid.Nil, errUserIDInvalidType
	}
	if userID == uuid.Nil {
		return uuid.Nil, errUserIDInvalidValue
	}
	return userID, nil
}

// SecurityVersionFromContext возвращает версию security-состояния пользователя из контекста.
func SecurityVersionFromContext(ctx context.Context) (int64, error) {
	if ctx == nil {
		return 0, errSecurityVersionMissing
	}
	value := ctx.Value(securityVersionContextKey)
	if value == nil {
		return 0, errSecurityVersionMissing
	}

	securityVersion, ok := value.(int64)
	if !ok {
		return 0, errSecurityVersionInvalidType
	}
	if securityVersion <= 0 {
		return 0, errSecurityVersionInvalidValue
	}
	return securityVersion, nil
}
