package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_ensureDirectory_AcceptsDirectory проверяет успешную проверку директории.
func Test_ensureDirectory_AcceptsDirectory(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	err := ensureDirectory(dir)

	// Assert
	require.NoError(t, err)
}

// Test_ensureDirectory_RejectsFile проверяет ошибку, когда путь указывает на файл.
func Test_ensureDirectory_RejectsFile(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(path, []byte("data"), 0o600))

	// Act
	err := ensureDirectory(path)

	// Assert
	require.Error(t, err)
	assert.Equal(t, "путь для сохранения файла должен быть директорией", err.Error())
}
