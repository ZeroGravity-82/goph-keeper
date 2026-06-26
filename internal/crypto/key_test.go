package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeriveKEK проверяет детерминированность вычисления KEK из мастер-ключа и соли.
func TestDeriveKEK(t *testing.T) {
	// Arrange
	masterKey := "hive tune oust abe seem rich"
	salt := []byte("1234567890abcdef")

	// Act
	firstKey, err := DeriveKEK(masterKey, salt)
	require.NoError(t, err)
	secondKey, err := DeriveKEK(masterKey, salt)
	require.NoError(t, err)

	// Assert
	assert.Len(t, firstKey, kekLength)
	assert.Equal(t, firstKey, secondKey)
}

// TestDeriveKEK_DifferentInput проверяет, что разные входные данные дают разные KEK.
func TestDeriveKEK_DifferentInput(t *testing.T) {
	// Arrange
	masterKey := "hive tune oust abe seem rich"
	salt := []byte("1234567890abcdef")
	otherSalt := []byte("abcdef1234567890")

	// Act
	key, err := DeriveKEK(masterKey, salt)
	require.NoError(t, err)
	keyWithOtherSalt, err := DeriveKEK(masterKey, otherSalt)
	require.NoError(t, err)
	keyWithOtherMasterKey, err := DeriveKEK("other master key", salt)
	require.NoError(t, err)

	// Assert
	assert.False(t, bytes.Equal(key, keyWithOtherSalt))
	assert.False(t, bytes.Equal(key, keyWithOtherMasterKey))
}

// TestDeriveKEK_FailWithInvalidInput проверяет ошибки при некорректных входных данных.
func TestDeriveKEK_FailWithInvalidInput(t *testing.T) {
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
			key, err := DeriveKEK(tt.masterKey, tt.salt)

			// Assert
			require.Error(t, err)
			assert.Nil(t, key)
		})
	}
}

// TestGenerateDEK проверяет генерацию ключа шифрования данных.
func TestGenerateDEK(t *testing.T) {
	// Act
	dek, err := GenerateDEK()

	// Assert
	require.NoError(t, err)
	assert.Len(t, dek, dekLength)
	assert.NotEqual(t, make([]byte, dekLength), dek)
}

// TestGenerateDEK_Unique проверяет, что два вызова генерируют разные DEK.
func TestGenerateDEK_Unique(t *testing.T) {
	// Act
	firstDEK, err := GenerateDEK()
	require.NoError(t, err)
	secondDEK, err := GenerateDEK()
	require.NoError(t, err)

	// Assert
	assert.False(t, bytes.Equal(firstDEK, secondDEK))
}
