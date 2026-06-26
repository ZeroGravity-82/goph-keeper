package crypto

import (
	"errors"

	"zerogravity-82/goph-keeper/internal/domain/model"
)

// EncryptDEK шифрует DEK с помощью KEK.
func EncryptDEK(dek []byte, kek []byte) (model.EncryptedBlob, error) {
	if len(dek) != dekLength {
		return model.EncryptedBlob{}, errors.New("DEK has invalid length")
	}
	return Encrypt(dek, kek)
}

// DecryptDEK расшифровывает DEK с помощью KEK.
func DecryptDEK(encryptedDEK model.EncryptedBlob, kek []byte) ([]byte, error) {
	dek, err := Decrypt(encryptedDEK, kek)
	if err != nil {
		return nil, err
	}
	if len(dek) != dekLength {
		return nil, errors.New("decrypted DEK has invalid length")
	}
	return dek, nil
}
