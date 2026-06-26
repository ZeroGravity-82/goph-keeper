package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEncryptDecryptDEK проверяет шифрование и расшифровку DEK через KEK.
func TestEncryptDecryptDEK(t *testing.T) {
	// Arrange
	dek, err := GenerateDEK()
	require.NoError(t, err)
	kek := testKEK(t)

	// Act
	encryptedDEK, err := EncryptDEK(dek, kek)
	require.NoError(t, err)
	decryptedDEK, err := DecryptDEK(encryptedDEK, kek)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, dek, decryptedDEK)
	assert.NotEqual(t, dek, encryptedDEK.Data)
}

// TestEncryptDEK_FailWithInvalidDEK проверяет ошибку при DEK некорректной длины.
func TestEncryptDEK_FailWithInvalidDEK(t *testing.T) {
	// Arrange
	kek := testKEK(t)

	// Act
	encryptedDEK, err := EncryptDEK([]byte("short-dek"), kek)

	// Assert
	require.Error(t, err)
	assert.Empty(t, encryptedDEK.Data)
}

// TestDecryptDEK_FailWithWrongKEK проверяет ошибку при расшифровке DEK неправильным KEK.
func TestDecryptDEK_FailWithWrongKEK(t *testing.T) {
	// Arrange
	dek, err := GenerateDEK()
	require.NoError(t, err)
	kek := testKEK(t)
	wrongKEK := bytes.Repeat([]byte{2}, kekLength)
	encryptedDEK, err := EncryptDEK(dek, kek)
	require.NoError(t, err)

	// Act
	decryptedDEK, err := DecryptDEK(encryptedDEK, wrongKEK)

	// Assert
	require.Error(t, err)
	assert.Nil(t, decryptedDEK)
}

// TestDecryptDEK_FailWithInvalidPlaintextLength проверяет ошибку, если расшифрованное значение не похоже на DEK.
func TestDecryptDEK_FailWithInvalidPlaintextLength(t *testing.T) {
	// Arrange
	kek := testKEK(t)
	encryptedNotDEK, err := Encrypt([]byte("not-a-dek"), kek)
	require.NoError(t, err)

	// Act
	decryptedDEK, err := DecryptDEK(encryptedNotDEK, kek)

	// Assert
	require.Error(t, err)
	assert.Nil(t, decryptedDEK)
}

func testKEK(t *testing.T) []byte {
	t.Helper()
	return bytes.Repeat([]byte{1}, kekLength)
}
