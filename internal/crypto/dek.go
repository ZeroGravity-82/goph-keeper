package crypto

import (
	"errors"

	"zerogravity-82/goph-keeper/internal/domain/model"
)

// encryptDEK шифрует DEK с помощью KEK.
func encryptDEK(dek []byte, kek []byte) (model.EncryptedBlob, error) {
	if len(dek) != dekSizeBytes {
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
	if len(dek) != dekSizeBytes {
		return nil, errors.New("decrypted DEK has invalid length")
	}
	return dek, nil
}

// ReencryptDEK расшифровывает DEK старым мастер-ключом и заново шифрует тот же DEK новым мастер-ключом.
func ReencryptDEK(
	oldMasterKey string,
	oldSalt []byte,
	newMasterKey string,
	newSalt []byte,
	encryptedDEK model.EncryptedBlob,
) (model.EncryptedBlob, error) {
	oldKEK, err := deriveKEK(oldMasterKey, oldSalt)
	if err != nil {
		return model.EncryptedBlob{}, err
	}
	dek, err := decryptDEK(encryptedDEK, oldKEK)
	if err != nil {
		return model.EncryptedBlob{}, err
	}
	newKEK, err := deriveKEK(newMasterKey, newSalt)
	if err != nil {
		return model.EncryptedBlob{}, err
	}
	return encryptDEK(dek, newKEK)
}
