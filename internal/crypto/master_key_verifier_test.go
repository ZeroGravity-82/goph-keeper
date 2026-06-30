package crypto

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/goph-keeper/internal/domain/model"
)

// TestMasterKeyVerifier проверяет создание и проверку верификатора мастер-ключа.
func TestMasterKeyVerifier(t *testing.T) {
	// Arrange
	salt := testMasterKeySalt()

	// Act
	verifier, err := EncryptMasterKeyVerifier("master-key", salt)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, verifier.Data)
	require.NoError(t, VerifyMasterKey("master-key", salt, verifier))
}

// TestMasterKeyVerifier_Unique проверяет, что верификатор шифруется с новым nonce при каждом вызове.
func TestMasterKeyVerifier_Unique(t *testing.T) {
	// Arrange
	salt := testMasterKeySalt()

	// Act
	first, err := EncryptMasterKeyVerifier("master-key", salt)
	require.NoError(t, err)
	second, err := EncryptMasterKeyVerifier("master-key", salt)
	require.NoError(t, err)

	// Assert
	assert.NotEqual(t, first.Data, second.Data)
	require.NoError(t, VerifyMasterKey("master-key", salt, first))
	require.NoError(t, VerifyMasterKey("master-key", salt, second))
}

// TestVerifyMasterKey_FailWithWrongMasterKey проверяет ошибку при неверном мастер-ключе.
func TestVerifyMasterKey_FailWithWrongMasterKey(t *testing.T) {
	// Arrange
	salt := testMasterKeySalt()
	verifier, err := EncryptMasterKeyVerifier("master-key", salt)
	require.NoError(t, err)

	// Act
	err = VerifyMasterKey("wrong-master-key", salt, verifier)

	// Assert
	require.ErrorIs(t, err, ErrInvalidMasterKey)
}

// TestVerifyMasterKey_FailWithEmptyVerifier проверяет ошибку при отсутствии верификатора.
func TestVerifyMasterKey_FailWithEmptyVerifier(t *testing.T) {
	// Act
	err := VerifyMasterKey("master-key", testMasterKeySalt(), model.EncryptedBlob{})

	// Assert
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrInvalidMasterKey))
}

func testMasterKeySalt() []byte {
	return []byte("1234567890abcdef")
}
