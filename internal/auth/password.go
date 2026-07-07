package auth

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword вычисляет bcrypt-хеш SHA-256-дайджеста пароля.
// Предварительный дайджест снимает 72-байтное ограничение bcrypt с исходного пароля.
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("failed to hash empty password")
	}
	b, err := bcrypt.GenerateFromPassword(passwordDigest(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(b), nil
}

// CheckPasswordHash сравнивает пароль с сохраненным bcrypt-хешем SHA-256-дайджеста пароля.
func CheckPasswordHash(password, passwordHash string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), passwordDigest(password)); err != nil {
		return fmt.Errorf("invalid password: %w", err)
	}
	return nil
}

func passwordDigest(password string) []byte {
	sum := sha256.Sum256([]byte(password))
	return sum[:]
}
