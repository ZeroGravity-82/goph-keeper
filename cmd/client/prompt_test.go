package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_normalizeInput_TrimsSpaces проверяет нормализацию пользовательского ввода.
func Test_normalizeInput_TrimsSpaces(t *testing.T) {
	// Act
	value, err := normalizeInput("  qwerty\n")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "qwerty", value)
}

// Test_normalizeInput_RejectsInvalidUTF8 проверяет, что невалидный UTF-8 не доходит до строковых gRPC-полей.
func Test_normalizeInput_RejectsInvalidUTF8(t *testing.T) {
	// Act
	_, err := normalizeInput(string([]byte{0xff, 0xfe}))

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "UTF-8")
}
