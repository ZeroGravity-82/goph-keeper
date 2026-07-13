package authcontext

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserIDFromContext_ReturnsStoredUserID проверяет чтение идентификатора пользователя из контекста.
func TestUserIDFromContext_ReturnsStoredUserID(t *testing.T) {
	// Arrange
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-000000000001")
	ctx := WithUserID(context.Background(), userID)

	// Act
	got, err := UserIDFromContext(ctx)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, userID, got)
}

// TestSecurityVersionFromContext_ReturnsStoredVersion проверяет чтение версии security-состояния пользователя из
// контекста.
func TestSecurityVersionFromContext_ReturnsStoredVersion(t *testing.T) {
	// Arrange
	userID := uuid.MustParse("018f6b7c-0000-7000-8000-000000000002")
	ctx := WithUserSession(context.Background(), userID, 3)

	// Act
	got, err := SecurityVersionFromContext(ctx)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, int64(3), got)
}

// TestUserIDFromContext_RejectsInvalidUserID проверяет ошибки чтения некорректного идентификатора пользователя.
func TestUserIDFromContext_RejectsInvalidUserID(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		wantErr error
	}{
		{
			name:    "nil context",
			ctx:     nil,
			wantErr: errUserIDMissing,
		},
		{
			name:    "missing user id",
			ctx:     context.Background(),
			wantErr: errUserIDMissing,
		},
		{
			name:    "invalid user id type",
			ctx:     context.WithValue(context.Background(), userIDContextKey, "not-a-uuid"),
			wantErr: errUserIDInvalidType,
		},
		{
			name:    "nil uuid",
			ctx:     WithUserID(context.Background(), uuid.Nil),
			wantErr: errUserIDInvalidValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := UserIDFromContext(tt.ctx)

			// Assert
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, uuid.Nil, got)
		})
	}
}

// TestSecurityVersionFromContext_RejectsInvalidVersion проверяет ошибки чтения некорректной версии security-состояния
// пользователя.
func TestSecurityVersionFromContext_RejectsInvalidVersion(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		wantErr error
	}{
		{
			name:    "nil context",
			ctx:     nil,
			wantErr: errSecurityVersionMissing,
		},
		{
			name:    "missing security version",
			ctx:     context.Background(),
			wantErr: errSecurityVersionMissing,
		},
		{
			name:    "invalid security version type",
			ctx:     context.WithValue(context.Background(), securityVersionContextKey, "1"),
			wantErr: errSecurityVersionInvalidType,
		},
		{
			name:    "zero security version",
			ctx:     WithUserSession(context.Background(), uuid.New(), 0),
			wantErr: errSecurityVersionInvalidValue,
		},
		{
			name:    "negative security version",
			ctx:     WithUserSession(context.Background(), uuid.New(), -1),
			wantErr: errSecurityVersionInvalidValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := SecurityVersionFromContext(tt.ctx)

			// Assert
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Zero(t, got)
		})
	}
}
