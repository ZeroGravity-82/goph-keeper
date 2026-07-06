package authcontext

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TestUserIDFromContext_ReturnsStoredUserID проверяет чтение идентификатора пользователя из контекста.
func TestUserIDFromContext_ReturnsStoredUserID(t *testing.T) {
	// Arrange
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-000000000001")
	ctx := WithUserID(context.Background(), userID)

	// Act
	got, ok := UserIDFromContext(ctx)

	// Assert
	assert.True(t, ok)
	assert.Equal(t, userID, got)
}

// TestSecurityVersionFromContext_ReturnsStoredVersion проверяет чтение версии security-состояния пользователя из
// контекста.
func TestSecurityVersionFromContext_ReturnsStoredVersion(t *testing.T) {
	// Arrange
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-000000000002")
	ctx := WithUserSession(context.Background(), userID, 3)

	// Act
	got, ok := SecurityVersionFromContext(ctx)

	// Assert
	assert.True(t, ok)
	assert.Equal(t, int64(3), got)
}

// TestUserIDFromContext_RejectsMissingUserID проверяет отсутствие пользователя в пустом или nil-контексте.
func TestUserIDFromContext_RejectsMissingUserID(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
	}{
		{name: "nil", ctx: nil},
		{name: "empty", ctx: context.Background()},
		{name: "nil uuid", ctx: WithUserID(context.Background(), uuid.Nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, ok := UserIDFromContext(tt.ctx)

			// Assert
			assert.False(t, ok)
			assert.Equal(t, uuid.Nil, got)
		})
	}
}

// TestSecurityVersionFromContext_RejectsMissingVersion проверяет отсутствие версии security-состояния пользователя в
// пустом или nil-контексте.
func TestSecurityVersionFromContext_RejectsMissingVersion(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
	}{
		{name: "nil", ctx: nil},
		{name: "empty", ctx: context.Background()},
		{name: "zero", ctx: WithUserSession(context.Background(), uuid.New(), 0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, ok := SecurityVersionFromContext(tt.ctx)

			// Assert
			assert.False(t, ok)
			assert.Zero(t, got)
		})
	}
}
