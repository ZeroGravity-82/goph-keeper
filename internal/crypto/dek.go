package crypto

import (
	"errors"

	"zerogravity-82/goph-keeper/internal/domain/model"
)

// encryptDEK шифрует DEK с помощью KEK.
func encryptDEK(dek []byte, kek []byte) (model.EncryptedBlob, error) {
	if len(dek) != dekLength {
		return model.EncryptedBlob{}, errors.New("DEK has invalid length")
	}
	return encrypt(dek, kek)
}

// decryptDEK расшифровывает DEK с помощью KEK.
func decryptDEK(encryptedDEK model.EncryptedBlob, kek []byte) ([]byte, error) {
	dek, err := decrypt(encryptedDEK, kek)
	if err != nil {
		return nil, err
	}
	if len(dek) != dekLength {
		return nil, errors.New("decrypted DEK has invalid length")
	}
	return dek, nil
}
