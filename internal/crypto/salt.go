package crypto

import (
	"crypto/rand"
	"fmt"
)

// GenerateMasterKeySalt генерирует соль для мастер-ключа.
func GenerateMasterKeySalt() ([]byte, error) {
	const masterKeySaltLength = 16

	s := make([]byte, masterKeySaltLength)
	if _, err := rand.Read(s); err != nil {
		return nil, fmt.Errorf("failed to generate master key salt: %w", err)
	}

	return s, nil
}
