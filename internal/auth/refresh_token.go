package auth

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashRefreshToken вычисляет хеш refresh-токена для хранения на сервере.
func HashRefreshToken(token string) string {
	s := sha256.Sum256([]byte(token))
	return hex.EncodeToString(s[:])
}
