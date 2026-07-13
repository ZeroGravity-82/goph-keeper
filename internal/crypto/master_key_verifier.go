package crypto

import (
	"errors"
	"fmt"

	"zerogravity-82/goph-keeper/internal/domain/model"
)

const masterKeyVerifierPlaintext = "goph-keeper-master-key-verifier-v1"

// ErrInvalidMasterKey возвращается, если верификатор не расшифровывается мастер-ключом.
var ErrInvalidMasterKey = errors.New("invalid master key")

// EncryptMasterKeyVerifier шифрует верификатор ключом KEK, полученным из мастер-ключа и соли.
func EncryptMasterKeyVerifier(masterKey string, salt []byte) (model.EncryptedBlob, error) {
	kek, err := deriveKEK(masterKey, salt)
	if err != nil {
		return model.EncryptedBlob{}, fmt.Errorf("failed to derive KEK: %w", err)
	}

	verifier, err := encrypt([]byte(masterKeyVerifierPlaintext), kek)
	if err != nil {
		return model.EncryptedBlob{}, fmt.Errorf("failed to encrypt master key verifier: %w", err)
	}
	return verifier, nil
}

// VerifyMasterKey проверяет мастер-ключ через расшифровку верификатора.
func VerifyMasterKey(masterKey string, salt []byte, verifier model.EncryptedBlob) error {
	if len(verifier.Data) == 0 {
		return errors.New("master key verifier is required")
	}

	kek, err := deriveKEK(masterKey, salt)
	if err != nil {
		return fmt.Errorf("failed to derive KEK: %w", err)
	}

	plaintext, err := decrypt(verifier, kek)
	if err != nil {
		return ErrInvalidMasterKey
	}
	if string(plaintext) != masterKeyVerifierPlaintext {
		return ErrInvalidMasterKey
	}
	return nil
}
