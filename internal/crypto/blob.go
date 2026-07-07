package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	"zerogravity-82/goph-keeper/internal/domain/model"
)

// nonceSizeBytes задает размер nonce для AES-GCM.
const nonceSizeBytes = 12

// aeadTagSizeBytes задает размер тега аутентификации AES-GCM.
const aeadTagSizeBytes = 16

// EncryptedBlobOverheadSizeBytes задает размер служебных данных, которые AES-GCM добавляет к открытому тексту: nonce и
// тег аутентификации.
const EncryptedBlobOverheadSizeBytes = int64(nonceSizeBytes + aeadTagSizeBytes)

// EncryptedBlobOverhead возвращает размер служебных данных, которые AES-GCM добавляет к открытому тексту.
func EncryptedBlobOverhead() int64 {
	return EncryptedBlobOverheadSizeBytes
}

// EncryptedChunkedBlobSize возвращает размер зашифрованных данных с учетом служебных данных AES-GCM для каждой части.
func EncryptedChunkedBlobSize(plainSize int64, plainChunkSize int64) (int64, error) {
	if plainSize <= 0 {
		return 0, errors.New("plain size is invalid")
	}
	if plainChunkSize <= 0 {
		return 0, errors.New("plain chunk size is invalid")
	}

	chunkCount := (plainSize + plainChunkSize - 1) / plainChunkSize
	return plainSize + chunkCount*EncryptedBlobOverhead(), nil
}

// encrypt шифрует данные ключом AES-256-GCM и возвращает blob в формате nonce + ciphertext + тег аутентификации.
func encrypt(plaintext []byte, key []byte) (model.EncryptedBlob, error) {
	return encryptWithAAD(plaintext, key, nil)
}

// encryptWithAAD шифрует данные ключом AES-256-GCM и включает associatedData в проверку подлинности, не сохраняя их в
// blob.
func encryptWithAAD(plaintext []byte, key []byte, associatedData []byte) (model.EncryptedBlob, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return model.EncryptedBlob{}, fmt.Errorf("failed to prepare cipher for encryption: %w", err)
	}

	nonce := make([]byte, nonceSizeBytes)
	if _, err = rand.Read(nonce); err != nil {
		return model.EncryptedBlob{}, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, associatedData)
	data := make([]byte, 0, len(nonce)+len(ciphertext))
	data = append(data, nonce...)
	data = append(data, ciphertext...)
	return model.EncryptedBlob{Data: data}, nil
}

// newGCM проверяет размер AES-256-ключа и создает AEAD-обертку AES-GCM.
func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != aes256KeySizeBytes {
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

// decrypt расшифровывает blob в формате nonce + ciphertext + тег аутентификации ключом AES-256-GCM.
func decrypt(blob model.EncryptedBlob, key []byte) ([]byte, error) {
	return decryptWithAAD(blob, key, nil)
}

// decryptWithAAD расшифровывает blob ключом AES-256-GCM и проверяет, что associatedData совпадают с данными,
// использованными при шифровании.
func decryptWithAAD(blob model.EncryptedBlob, key []byte, associatedData []byte) ([]byte, error) {
	if len(blob.Data) < nonceSizeBytes {
		return nil, errors.New("encrypted blob is too short")
	}

	gcm, err := newGCM(key)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare cipher for decryption: %w", err)
	}

	nonce := blob.Data[:nonceSizeBytes]
	ciphertext := blob.Data[nonceSizeBytes:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, associatedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt blob: %w", err)
	}
	return plaintext, nil
}
