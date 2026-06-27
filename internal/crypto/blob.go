package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	"zerogravity-82/goph-keeper/internal/domain/model"
)

const nonceLength = 12

// encrypt шифрует данные ключом AES-256-GCM и возвращает blob в формате nonce + ciphertext.
func encrypt(plaintext []byte, key []byte) (model.EncryptedBlob, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return model.EncryptedBlob{}, fmt.Errorf("failed to prepare cipher for encryption: %w", err)
	}

	nonce := make([]byte, nonceLength)
	if _, err = rand.Read(nonce); err != nil {
		return model.EncryptedBlob{}, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	data := make([]byte, 0, len(nonce)+len(ciphertext))
	data = append(data, nonce...)
	data = append(data, ciphertext...)
	return model.EncryptedBlob{Data: data}, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != aes256KeyLength {
		return nil, errors.New("AES-256 key has invalid length")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	return gcm, nil
}

// decrypt расшифровывает blob в формате nonce + ciphertext ключом AES-256-GCM.
func decrypt(blob model.EncryptedBlob, key []byte) ([]byte, error) {
	if len(blob.Data) < nonceLength {
		return nil, errors.New("encrypted blob is too short")
	}

	gcm, err := newGCM(key)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare cipher for decryption: %w", err)
	}

	nonce := blob.Data[:nonceLength]
	ciphertext := blob.Data[nonceLength:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt blob: %w", err)
	}
	return plaintext, nil
}
