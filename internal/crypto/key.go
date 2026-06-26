package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
)

const (
	// masterKeySaltLength задает длину пользовательской соли для вычисления KEK.
	masterKeySaltLength = 16

	// kekLength задает длину KEK для AES-256.
	kekLength = 32

	// dekLength задает длину DEK для AES-256.
	dekLength = 32

	// argon2IDTime задает число проходов Argon2id.
	argon2IDTime = 3

	// argon2IDMemory задает объем памяти Argon2id в килобайтах.
	argon2IDMemory = 64 * 1024

	// argon2IDParallelism задает число параллельных потоков Argon2id.
	argon2IDParallelism = 2
)

// DeriveKEK вычисляет ключ шифрования ключей из мастер-ключа и пользовательской соли.
func DeriveKEK(masterKey string, salt []byte) ([]byte, error) {
	if masterKey == "" {
		return nil, errors.New("master key is required")
	}
	if len(salt) != masterKeySaltLength {
		return nil, errors.New("master key salt has invalid length")
	}

	key := argon2.IDKey(
		[]byte(masterKey),
		salt,
		argon2IDTime,
		argon2IDMemory,
		argon2IDParallelism,
		kekLength,
	)
	return key, nil
}

// GenerateDEK генерирует ключ шифрования данных для приватной записи.
func GenerateDEK() ([]byte, error) {
	key := make([]byte, dekLength)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate DEK: %w", err)
	}

	return key, nil
}
