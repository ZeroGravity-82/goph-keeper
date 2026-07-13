package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// GenerateRefreshToken возвращает криптографически стойкий непрозрачный refresh-токен.
//
// Токен предполагается хранить на стороне клиента и обменивать на новый access-токен. Его необходимо считать секретом.
func (m *TokenManager) GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// URL-safe without padding.
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashRefreshToken вычисляет хеш refresh-токена для хранения на сервере.
func HashRefreshToken(token string) string {
	s := sha256.Sum256([]byte(token))
	return hex.EncodeToString(s[:])
}
