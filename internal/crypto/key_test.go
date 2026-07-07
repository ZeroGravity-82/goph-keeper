package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_deriveKEK проверяет детерминированность вычисления KEK из мастер-ключа и соли.
func Test_deriveKEK(t *testing.T) {
	// Arrange
	masterKey := "hive tune oust abe seem rich"
	salt := []byte("1234567890abcdef")

	// Act
	firstKey, err := deriveKEK(masterKey, salt)
	require.NoError(t, err)
	secondKey, err := deriveKEK(masterKey, salt)
	require.NoError(t, err)

	// Assert
	assert.Len(t, firstKey, kekSizeBytes)
	assert.Equal(t, firstKey, secondKey)
}

// Test_deriveKEK_DifferentInput проверяет, что разные входные данные дают разные KEK.
func Test_deriveKEK_DifferentInput(t *testing.T) {
	// Arrange
	masterKey := "hive tune oust abe seem rich"
	salt := []byte("1234567890abcdef")
	otherSalt := []byte("abcdef1234567890")

	// Act
	key, err := deriveKEK(masterKey, salt)
	require.NoError(t, err)
	keyWithOtherSalt, err := deriveKEK(masterKey, otherSalt)
	require.NoError(t, err)
	keyWithOtherMasterKey, err := deriveKEK("other master key", salt)
	require.NoError(t, err)

	// Assert
	assert.False(t, bytes.Equal(key, keyWithOtherSalt))
	assert.False(t, bytes.Equal(key, keyWithOtherMasterKey))
}

// Test_deriveKEK_FailWithInvalidInput проверяет ошибки при некорректных входных данных.
func Test_deriveKEK_FailWithInvalidInput(t *testing.T) {
	// Arrange
	tests := []struct {
		name      string
		masterKey string
		salt      []byte
	}{
		{name: "empty master key", masterKey: "", salt: []byte("1234567890abcdef")},
		{name: "empty salt", masterKey: "master key", salt: nil},
		{name: "short salt", masterKey: "master key", salt: []byte("short")},
		{name: "long salt", masterKey: "master key", salt: []byte("1234567890abcdef-extra")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			key, err := deriveKEK(tt.masterKey, tt.salt)

			// Assert
			require.Error(t, err)
			assert.Nil(t, key)
		})
	}
}

// Test_generateDEK проверяет генерацию ключа шифрования данных.
func Test_generateDEK(t *testing.T) {
	// Act
	dek, err := generateDEK()

	// Assert
	require.NoError(t, err)
	assert.Len(t, dek, dekSizeBytes)
	assert.NotEqual(t, make([]byte, dekSizeBytes), dek)
}

// Test_generateDEK_Unique проверяет, что два вызова генерируют разные DEK.
func Test_generateDEK_Unique(t *testing.T) {
	// Act
	firstDEK, err := generateDEK()
	require.NoError(t, err)
	secondDEK, err := generateDEK()
	require.NoError(t, err)

	// Assert
	assert.False(t, bytes.Equal(firstDEK, secondDEK))
}
