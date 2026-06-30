package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"
)

// masterKeySaltLength задает длину пользовательской соли для вычисления KEK.
const masterKeySaltLength = 16

// ValidateMasterKeySalt проверяет длину соли мастер-ключа.
func ValidateMasterKeySalt(salt []byte) error {
	if len(salt) != masterKeySaltLength {
		return errors.New("master key salt has invalid length")
	}
	return nil
}

// GenerateMasterKeySalt генерирует соль для мастер-ключа.
func GenerateMasterKeySalt() ([]byte, error) {
	s := make([]byte, masterKeySaltLength)
	if _, err := rand.Read(s); err != nil {
		return nil, fmt.Errorf("failed to generate master key salt: %w", err)
	}

	return s, nil
}
