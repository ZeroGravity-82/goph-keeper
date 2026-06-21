package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewTokenManager проверяет создание TokenManager и валидацию входных параметров.
func TestNewTokenManager(t *testing.T) {
	// Arrange
	tests := []struct {
		name             string
		secret           string
		accessTokenTTL   time.Duration
		wantTokenManager TokenManager
		wantErr          bool
	}{
		{
			name:             "fail with empty secret",
			secret:           "",
			accessTokenTTL:   15 * time.Minute,
			wantTokenManager: TokenManager{},
			wantErr:          true,
		},
		{
			name:             "fail with zero access token TTL",
			secret:           "secret",
			accessTokenTTL:   0,
			wantTokenManager: TokenManager{},
			wantErr:          true,
		},
		{
			name:             "fail with negative access token TTL",
			secret:           "secret",
			accessTokenTTL:   -15 * time.Minute,
			wantTokenManager: TokenManager{},
			wantErr:          true,
		},
		{
			name:           "can create token manager",
			secret:         "secret",
			accessTokenTTL: 15 * time.Minute,
			wantTokenManager: TokenManager{
				secret:         []byte("secret"),
				accessTokenTTL: 15 * time.Minute,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			m, err := NewTokenManager(tt.secret, tt.accessTokenTTL)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, m)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantTokenManager, *m)
			}

		})
	}
}

// TestIssueAndParseAccessToken_OK проверяет выпуск access-токена и последующий разбор claims.
func TestIssueAndParseAccessToken_OK(t *testing.T) {
	// Arrange
	m, err := NewTokenManager("secret", 15*time.Minute)
	require.NoError(t, err)
	userID, err := uuid.NewV7()
	require.NoError(t, err)

	// Act
	tok, err := m.IssueAccessToken(userID)
	require.NoError(t, err)
	assert.NotEmpty(t, tok)

	claims, err := m.ParseAccessToken(tok)
	require.NoError(t, err)

	// Assert
	assert.Equal(t, userID.String(), claims.Subject)
	assert.NotNil(t, claims.IssuedAt)
	assert.NotNil(t, claims.ExpiresAt)
	assert.True(t, claims.ExpiresAt.After(claims.IssuedAt.Time))
}

// TestParseAccessToken_FailWithInvalidToken проверяет ошибку при разборе некорректного токена.
func TestParseAccessToken_FailWithInvalidToken(t *testing.T) {
	// Arrange
	m, err := NewTokenManager("secret", 15*time.Minute)
	require.NoError(t, err)

	// Act
	_, err = m.ParseAccessToken("not-a-jwt")

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

// TestParseAccessToken_FailWithWrongSecret проверяет ошибку, если токен подписан другим секретом.
func TestParseAccessToken_FailWithWrongSecret(t *testing.T) {
	// Arrange
	m1, err := NewTokenManager("secret-1", 15*time.Minute)
	require.NoError(t, err)
	m2, err := NewTokenManager("secret-2", 15*time.Minute)
	require.NoError(t, err)
	userID, err := uuid.NewV7()
	require.NoError(t, err)

	tok, err := m1.IssueAccessToken(userID)
	require.NoError(t, err)

	// Act
	_, err = m2.ParseAccessToken(tok)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

// TestParseAccessToken_FailWithExpiredToken проверяет ошибку при разборе истекшего токена.
func TestParseAccessToken_FailWithExpiredToken(t *testing.T) {
	// Arrange
	m, err := NewTokenManager("secret", 1*time.Millisecond)
	require.NoError(t, err)
	userID, err := uuid.NewV7()
	require.NoError(t, err)

	tok, err := m.IssueAccessToken(userID)
	require.NoError(t, err)

	// Гарантируем, что токен истек
	time.Sleep(10 * time.Millisecond)

	// Act
	_, err = m.ParseAccessToken(tok)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

// TestParseAccessToken_FailWithRejectedNonHMACAlg проверяет отклонение токена с неподдерживаемым алгоритмом подписи.
func TestParseAccessToken_FailWithRejectedNonHMACAlg(t *testing.T) {
	// Arrange
	m, err := NewTokenManager("secret", 15*time.Minute)
	require.NoError(t, err)

	claims := AccessClaims{RegisteredClaims: jwt.RegisteredClaims{Subject: "user-123"}}
	// Подписываем методом "none", чтобы гарантировать, что ParseWithClaims увидит не HMAC-алгоритм.
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tok, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	// Act
	_, err = m.ParseAccessToken(tok)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidToken)
}
