package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

var (
	// ErrInvalidToken возвращается, когда JWT не удается распарсить или провалидировать.
	ErrInvalidToken = errors.New("invalid token")
)

// TokenManager выпускает и валидирует access JWT, а также генерирует криптографически стойкий непрозрачный
// refresh-токен.
//
// Для JWT используется подпись HMAC-SHA256 (HS256) с заданным секретом.
type TokenManager struct {
	secret         []byte
	accessTokenTTL time.Duration
}

// NewTokenManager создает TokenManager.
//
// secret используется для подписи/проверки токенов.
// accessTokenTTL задает время жизни access-токена.
func NewTokenManager(secret string, accessTokenTTL time.Duration) (*TokenManager, error) {
	if secret == "" {
		return nil, errors.New("JWT secret is empty")
	}
	if accessTokenTTL <= 0 {
		return nil, errors.New("access token TTL must be positive")
	}
	return &TokenManager{secret: []byte(secret), accessTokenTTL: accessTokenTTL}, nil
}

// AccessClaims содержит утверждения access JWT.
//
// Subject (sub) используется как идентификатор пользователя, SecurityVersion - как версия security-состояния пользователя.
type AccessClaims struct {
	jwt.RegisteredClaims
	SecurityVersion int64 `json:"security_version"`
}

// IssueAccessToken создает и подписывает новый access-токен для указанного userID.
//
// Возвращаемое значение — компактная строка JWT, подходящая для заголовка `Authorization: Bearer <token>`.
func (m *TokenManager) IssueAccessToken(userID uuid.UUID, securityVersion int64) (string, error) {
	if securityVersion <= 0 {
		return "", errors.New("security version must be positive")
	}
	now := time.Now().UTC()
	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTokenTTL)),
		},
		SecurityVersion: securityVersion,
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := t.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("failed to sign access token: %w", err)
	}
	return tokenString, nil
}

// ParseAccessToken парсит и валидирует строку токена, после чего возвращает распарсенные утверждения.
//
// В случае любых ошибок парсинга/валидации/подписи возвращает ErrInvalidToken.
func (m *TokenManager) ParseAccessToken(tokenString string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected access token signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(*AccessClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
