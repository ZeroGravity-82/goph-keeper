package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/goph-keeper/internal/domain/model"
)

// TestEncryptionRoundTrip проверяет шифрование и расшифровку данных.
func TestEncryptionRoundTrip(t *testing.T) {
	// Arrange
	key := testDEK(t)
	plaintext := []byte("secret payload")

	// Act
	blob, err := encrypt(plaintext, key)
	require.NoError(t, err)
	decrypted, err := decrypt(blob, key)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
	assert.Len(t, blob.Data, nonceSizeBytes+len(plaintext)+16)
	assert.NotEqual(t, plaintext, blob.Data)
}

// TestEncryptionRoundTrip_EmptyPlaintext проверяет, что пустой plaintext допустим.
func TestEncryptionRoundTrip_EmptyPlaintext(t *testing.T) {
	// Arrange
	key := testDEK(t)

	// Act
	blob, err := encrypt(nil, key)
	require.NoError(t, err)
	decrypted, err := decrypt(blob, key)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, decrypted)
	assert.Len(t, blob.Data, nonceSizeBytes+16)
}

// Test_encrypt_UniqueNonce проверяет, что повторное шифрование одного plaintext дает разные blob.
func Test_encrypt_UniqueNonce(t *testing.T) {
	// Arrange
	key := testDEK(t)
	plaintext := []byte("secret payload")

	// Act
	firstBlob, err := encrypt(plaintext, key)
	require.NoError(t, err)
	secondBlob, err := encrypt(plaintext, key)
	require.NoError(t, err)

	// Assert
	assert.False(t, bytes.Equal(firstBlob.Data, secondBlob.Data))
}

// Test_encrypt_FailWithInvalidKey проверяет ошибку шифрования при ключе некорректной длины.
func Test_encrypt_FailWithInvalidKey(t *testing.T) {
	// Arrange
	invalidKey := []byte("short-key")

	// Act
	encrypted, encryptErr := encrypt([]byte("payload"), invalidKey)

	// Assert
	require.Error(t, encryptErr)
	assert.Empty(t, encrypted.Data)
}

// Test_decrypt_FailWithInvalidKey проверяет ошибку расшифровывания при ключе некорректной длины.
func Test_decrypt_FailWithInvalidKey(t *testing.T) {
	// Arrange
	invalidKey := []byte("short-key")
	blob := model.EncryptedBlob{Data: bytes.Repeat([]byte{1}, nonceSizeBytes+16)}
	encrypted, encryptErr := encrypt([]byte("payload"), invalidKey)

	// Act
	decrypted, decryptErr := decrypt(blob, invalidKey)

	// Assert
	require.Error(t, encryptErr)
	assert.Empty(t, encrypted.Data)
	require.Error(t, decryptErr)
	assert.Nil(t, decrypted)
}

// Test_decrypt_FailWithWrongKey проверяет ошибку при попытке расшифровать blob другим ключом.
func Test_decrypt_FailWithWrongKey(t *testing.T) {
	// Arrange
	key := testDEK(t)
	wrongKey := bytes.Repeat([]byte{2}, dekSizeBytes)
	blob, err := encrypt([]byte("secret payload"), key)
	require.NoError(t, err)

	// Act
	decrypted, err := decrypt(blob, wrongKey)

	// Assert
	require.Error(t, err)
	assert.Nil(t, decrypted)
}

// Test_decrypt_FailWithDamagedCiphertext проверяет ошибку при поврежденном ciphertext.
func Test_decrypt_FailWithDamagedCiphertext(t *testing.T) {
	// Arrange
	key := testDEK(t)
	blob, err := encrypt([]byte("secret payload"), key)
	require.NoError(t, err)
	blob.Data[len(blob.Data)-1] ^= 1

	// Act
	decrypted, err := decrypt(blob, key)

	// Assert
	require.Error(t, err)
	assert.Nil(t, decrypted)
}

// Test_decrypt_FailWithShortBlob проверяет ошибку при blob короче nonce.
func Test_decrypt_FailWithShortBlob(t *testing.T) {
	// Arrange
	key := testDEK(t)
	blob := model.EncryptedBlob{Data: bytes.Repeat([]byte{1}, nonceSizeBytes-1)}

	// Act
	decrypted, err := decrypt(blob, key)

	// Assert
	require.Error(t, err)
	assert.Nil(t, decrypted)
}

func testDEK(t *testing.T) []byte {
	t.Helper()
	key, err := generateDEK()
	require.NoError(t, err)
	return key
}
