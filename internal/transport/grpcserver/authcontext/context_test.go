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
