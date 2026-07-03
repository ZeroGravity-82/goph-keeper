package main

import (
	"bufio"
	"bytes"
	"errors"
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

// Test_confirmOutputFileOverwrite_SkipsMissingFile проверяет, что для нового файла подтверждение не запрашивается.
func Test_confirmOutputFileOverwrite_SkipsMissingFile(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "file.txt")
	reader := bufio.NewReader(bytes.NewBufferString(""))
	out := bytes.NewBuffer(nil)

	// Act
	err := confirmOutputFileOverwrite(reader, out, path)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, out.String())
}

// Test_confirmOutputFileOverwrite_RejectsExistingDirectory проверяет ошибку, если целевой путь является директорией.
func Test_confirmOutputFileOverwrite_RejectsExistingDirectory(t *testing.T) {
	// Arrange
	path := t.TempDir()
	reader := bufio.NewReader(bytes.NewBufferString(""))
	out := bytes.NewBuffer(nil)

	// Act
	err := confirmOutputFileOverwrite(reader, out, path)

	// Assert
	require.Error(t, err)
	assert.Equal(t, "путь для сохранения файла уже существует и является директорией", err.Error())
}

// Test_confirmOutputFileOverwrite_CancelsOverwrite проверяет отказ от перезаписи существующего файла.
func Test_confirmOutputFileOverwrite_CancelsOverwrite(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(path, []byte("data"), 0o600))
	reader := bufio.NewReader(bytes.NewBufferString("n\n"))
	out := bytes.NewBuffer(nil)

	// Act
	err := confirmOutputFileOverwrite(reader, out, path)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, errActionCanceled))
	assert.Contains(t, out.String(), "Перезаписать?")
}

// Test_confirmOutputFileOverwrite_AllowsOverwrite проверяет подтверждение перезаписи существующего файла.
func Test_confirmOutputFileOverwrite_AllowsOverwrite(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(path, []byte("data"), 0o600))
	reader := bufio.NewReader(bytes.NewBufferString("y\n"))
	out := bytes.NewBuffer(nil)

	// Act
	err := confirmOutputFileOverwrite(reader, out, path)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Перезаписать?")
}
