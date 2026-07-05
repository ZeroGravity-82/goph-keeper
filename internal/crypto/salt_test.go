package crypto

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateMasterKeySalt проверяет валидацию длины соли мастер-ключа.
func TestValidateMasterKeySalt(t *testing.T) {
	tests := []struct {
		name    string
		salt    []byte
		wantErr bool
	}{
		{name: "valid", salt: []byte("1234567890abcdef")},
		{name: "empty", wantErr: true},
		{name: "short", salt: []byte("short"), wantErr: true},
		{name: "long", salt: []byte(strings.Repeat("a", masterKeySaltLength+1)), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			err := ValidateMasterKeySalt(tt.salt)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, "master key salt has invalid length", err.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestGenerateMasterKeySalt проверяет генерацию соли мастер-ключа.
func TestGenerateMasterKeySalt(t *testing.T) {
	// Act
	salt, err := GenerateMasterKeySalt()

	// Assert
	require.NoError(t, err)
	assert.Len(t, salt, 16)
	assert.NotEqual(t, make([]byte, 16), salt)
}

// TestGenerateMasterKeySalt_Unique проверяет, что два вызова генерируют разные соли.
func TestGenerateMasterKeySalt_Unique(t *testing.T) {
	// Act
	firstSalt, err := GenerateMasterKeySalt()
	require.NoError(t, err)
	secondSalt, err := GenerateMasterKeySalt()
	require.NoError(t, err)

	// Assert
	assert.False(t, bytes.Equal(firstSalt, secondSalt))
}
