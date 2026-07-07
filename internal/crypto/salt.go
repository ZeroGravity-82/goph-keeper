package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"
)

// masterKeySaltSizeBytes задает размер пользовательской соли для вычисления KEK.
const masterKeySaltSizeBytes = 16

// ValidateMasterKeySalt проверяет длину соли мастер-ключа.
func ValidateMasterKeySalt(salt []byte) error {
	if len(salt) != masterKeySaltSizeBytes {
		return errors.New("master key salt has invalid length")
	}
	return nil
}

// GenerateMasterKeySalt генерирует соль для мастер-ключа.
func GenerateMasterKeySalt() ([]byte, error) {
	s := make([]byte, masterKeySaltSizeBytes)
	if _, err := rand.Read(s); err != nil {
		return nil, fmt.Errorf("failed to generate master key salt: %w", err)
	}

	return s, nil
}
