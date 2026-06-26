package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/goph-keeper/internal/domain/model"
)

// TestEncryptDecrypt проверяет шифрование и расшифровку данных.
func TestEncryptDecrypt(t *testing.T) {
	// Arrange
	key := testKey(t)
	plaintext := []byte("secret payload")

	// Act
	blob, err := Encrypt(plaintext, key)
	require.NoError(t, err)
	decrypted, err := Decrypt(blob, key)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
	assert.Len(t, blob.Data, nonceLength+len(plaintext)+16)
	assert.NotEqual(t, plaintext, blob.Data)
}

// TestEncryptDecrypt_EmptyPlaintext проверяет, что пустой plaintext допустим.
func TestEncryptDecrypt_EmptyPlaintext(t *testing.T) {
	// Arrange
	key := testKey(t)

	// Act
	blob, err := Encrypt(nil, key)
	require.NoError(t, err)
	decrypted, err := Decrypt(blob, key)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, decrypted)
	assert.Len(t, blob.Data, nonceLength+16)
}

// TestEncrypt_UniqueNonce проверяет, что повторное шифрование одного plaintext дает разные blob.
func TestEncrypt_UniqueNonce(t *testing.T) {
	// Arrange
	key := testKey(t)
	plaintext := []byte("secret payload")

	// Act
	firstBlob, err := Encrypt(plaintext, key)
	require.NoError(t, err)
	secondBlob, err := Encrypt(plaintext, key)
	require.NoError(t, err)

	// Assert
	assert.False(t, bytes.Equal(firstBlob.Data, secondBlob.Data))
}

// TestEncryptDecrypt_FailWithInvalidKey проверяет ошибки при ключе некорректной длины.
func TestEncryptDecrypt_FailWithInvalidKey(t *testing.T) {
	// Arrange
	invalidKey := []byte("short-key")
	blob := model.EncryptedBlob{Data: bytes.Repeat([]byte{1}, nonceLength+16)}

	// Act
	encrypted, encryptErr := Encrypt([]byte("payload"), invalidKey)
	decrypted, decryptErr := Decrypt(blob, invalidKey)

	// Assert
	require.Error(t, encryptErr)
	assert.Empty(t, encrypted.Data)
	require.Error(t, decryptErr)
	assert.Nil(t, decrypted)
}

// TestDecrypt_FailWithWrongKey проверяет ошибку при попытке расшифровать blob другим ключом.
func TestDecrypt_FailWithWrongKey(t *testing.T) {
	// Arrange
	key := testKey(t)
	wrongKey := bytes.Repeat([]byte{2}, dekLength)
	blob, err := Encrypt([]byte("secret payload"), key)
	require.NoError(t, err)

	// Act
	decrypted, err := Decrypt(blob, wrongKey)

	// Assert
	require.Error(t, err)
	assert.Nil(t, decrypted)
}

// TestDecrypt_FailWithDamagedCiphertext проверяет ошибку при поврежденном ciphertext.
func TestDecrypt_FailWithDamagedCiphertext(t *testing.T) {
	// Arrange
	key := testKey(t)
	blob, err := Encrypt([]byte("secret payload"), key)
	require.NoError(t, err)
	blob.Data[len(blob.Data)-1] ^= 1

	// Act
	decrypted, err := Decrypt(blob, key)

	// Assert
	require.Error(t, err)
	assert.Nil(t, decrypted)
}

// TestDecrypt_FailWithShortBlob проверяет ошибку при blob короче nonce.
func TestDecrypt_FailWithShortBlob(t *testing.T) {
	// Arrange
	key := testKey(t)
	blob := model.EncryptedBlob{Data: bytes.Repeat([]byte{1}, nonceLength-1)}

	// Act
	decrypted, err := Decrypt(blob, key)

	// Assert
	require.Error(t, err)
	assert.Nil(t, decrypted)
}

func testKey(t *testing.T) []byte {
	t.Helper()
	key, err := GenerateDEK()
	require.NoError(t, err)
	return key
}
