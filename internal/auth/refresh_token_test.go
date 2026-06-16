package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHashRefreshToken проверяет вычисление SHA-256 хеша refresh-токена в hex-представлении.
func TestHashRefreshToken(t *testing.T) {
	// Arrange
	token := "refresh-token"
	wantSum := sha256.Sum256([]byte(token))
	wantHash := hex.EncodeToString(wantSum[:])

	// Act
	gotHash := HashRefreshToken(token)

	// Assert
	assert.Equal(t, wantHash, gotHash)
	assert.Len(t, gotHash, 64)
}

// TestHashRefreshToken_Deterministic проверяет, что один и тот же refresh-токен дает один и тот же хеш.
func TestHashRefreshToken_Deterministic(t *testing.T) {
	// Arrange
	token := "refresh-token"

	// Act
	firstHash := HashRefreshToken(token)
	secondHash := HashRefreshToken(token)

	// Assert
	assert.Equal(t, firstHash, secondHash)
}

// TestHashRefreshToken_DifferentTokens проверяет, что разные refresh-токены дают разные хеши.
func TestHashRefreshToken_DifferentTokens(t *testing.T) {
	// Act
	firstHash := HashRefreshToken("refresh-token-1")
	secondHash := HashRefreshToken("refresh-token-2")

	// Assert
	assert.NotEqual(t, firstHash, secondHash)
}
