package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEncryptDecryptDEKRoundTrip проверяет шифрование и расшифровку DEK через KEK.
func TestEncryptDecryptDEKRoundTrip(t *testing.T) {
	// Arrange
	dek, err := generateDEK()
	require.NoError(t, err)
	kek := testKEK(t)

	// Act
	encryptedDEK, err := encryptDEK(dek, kek)
	require.NoError(t, err)
	decryptedDEK, err := decryptDEK(encryptedDEK, kek)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, dek, decryptedDEK)
	assert.NotEqual(t, dek, encryptedDEK.Data)
}

// Test_encryptDEK_FailWithInvalidDEK проверяет ошибку при DEK некорректной длины.
func Test_encryptDEK_FailWithInvalidDEK(t *testing.T) {
	// Arrange
	kek := testKEK(t)

	// Act
	encryptedDEK, err := encryptDEK([]byte("short-dek"), kek)

	// Assert
	require.Error(t, err)
	assert.Empty(t, encryptedDEK.Data)
}

// Test_decryptDEK_FailWithWrongKEK проверяет ошибку при расшифровке DEK неправильным KEK.
func Test_decryptDEK_FailWithWrongKEK(t *testing.T) {
	// Arrange
	dek, err := generateDEK()
	require.NoError(t, err)
	kek := testKEK(t)
	wrongKEK := bytes.Repeat([]byte{2}, kekSizeBytes)
	encryptedDEK, err := encryptDEK(dek, kek)
	require.NoError(t, err)

	// Act
	decryptedDEK, err := decryptDEK(encryptedDEK, wrongKEK)

	// Assert
	require.Error(t, err)
	assert.Nil(t, decryptedDEK)
}

// TestReencryptDEK проверяет переупаковку DEK с одного мастер-ключа на другой.
func TestReencryptDEK(t *testing.T) {
	// Arrange
	dek, err := generateDEK()
	require.NoError(t, err)
	oldMasterKey := "old master key"
	oldSalt := []byte("1234567890abcdef")
	newMasterKey := "new master key"
	newSalt := []byte("abcdef1234567890")
	oldKEK, err := deriveKEK(oldMasterKey, oldSalt)
	require.NoError(t, err)
	encryptedDEK, err := encryptDEK(dek, oldKEK)
	require.NoError(t, err)

	// Act
	reencryptedDEK, err := ReencryptDEK(oldMasterKey, oldSalt, newMasterKey, newSalt, encryptedDEK)

	// Assert
	require.NoError(t, err)
	newKEK, err := deriveKEK(newMasterKey, newSalt)
	require.NoError(t, err)
	decryptedDEK, err := decryptDEK(reencryptedDEK, newKEK)
	require.NoError(t, err)
	assert.Equal(t, dek, decryptedDEK)

	_, err = decryptDEK(reencryptedDEK, oldKEK)
	require.Error(t, err)
}

// Test_decryptDEK_FailWithInvalidPlaintextLength проверяет ошибку, если расшифрованное значение не похоже на DEK.
func Test_decryptDEK_FailWithInvalidPlaintextLength(t *testing.T) {
	// Arrange
	kek := testKEK(t)
	encryptedNotDEK, err := encrypt([]byte("not-a-dek"), kek)
	require.NoError(t, err)

	// Act
	decryptedDEK, err := decryptDEK(encryptedNotDEK, kek)

	// Assert
	require.Error(t, err)
	assert.Nil(t, decryptedDEK)
}

func testKEK(t *testing.T) []byte {
	t.Helper()
	return bytes.Repeat([]byte{1}, kekSizeBytes)
}
