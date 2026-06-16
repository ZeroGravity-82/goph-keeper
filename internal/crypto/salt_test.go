package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
