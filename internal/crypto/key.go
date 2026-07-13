package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
)

const (
	// aes256KeySizeBytes задает размер ключа AES-256.
	aes256KeySizeBytes = 32

	// kekSizeBytes задает размер KEK; KEK используется как AES-256-ключ для шифрования DEK.
	kekSizeBytes = aes256KeySizeBytes

	// dekSizeBytes задает размер DEK; DEK используется как AES-256-ключ для шифрования payload.
	dekSizeBytes = aes256KeySizeBytes

	// argon2IDTime задает число проходов Argon2id.
	argon2IDTime = 3

	// argon2IDMemory задает объем памяти Argon2id в КиБ.
	argon2IDMemory = 64 * 1024

	// argon2IDParallelism задает число параллельных потоков Argon2id.
	argon2IDParallelism = 2
)

// deriveKEK вычисляет ключ шифрования ключей из мастер-ключа и пользовательской соли.
func deriveKEK(masterKey string, salt []byte) ([]byte, error) {
	if masterKey == "" {
		return nil, errors.New("master key is required")
	}
	if len(salt) != masterKeySaltSizeBytes {
		return nil, errors.New("master key salt has invalid length")
	}

	key := argon2.IDKey(
		[]byte(masterKey),
		salt,
		argon2IDTime,
		argon2IDMemory,
		argon2IDParallelism,
		kekSizeBytes,
	)
	return key, nil
}

// generateDEK генерирует ключ шифрования данных для приватной записи.
func generateDEK() ([]byte, error) {
	key := make([]byte, dekSizeBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate DEK: %w", err)
	}

	return key, nil
}
