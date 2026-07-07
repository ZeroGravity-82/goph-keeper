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

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
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

// Test_openInputFile_ReturnsFileAndSize проверяет открытие исходного файла для загрузки.
func Test_openInputFile_ReturnsFileAndSize(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(path, []byte("data"), 0o600))

	// Act
	file, size, err := openInputFile(path)

	// Assert
	require.NoError(t, err)
	defer func() {
		require.NoError(t, file.Close())
	}()
	assert.Equal(t, int64(4), size)
}

// Test_openInputFile_RejectsDirectory проверяет ошибку, если путь загрузки указывает на директорию.
func Test_openInputFile_RejectsDirectory(t *testing.T) {
	// Arrange
	path := t.TempDir()

	// Act
	file, _, err := openInputFile(path)

	// Assert
	require.Error(t, err)
	assert.Nil(t, file)
	assert.Equal(t, "путь к файлу указывает на директорию", err.Error())
}

// Test_openInputFile_FailWithMissingFile проверяет ошибку открытия отсутствующего файла.
func Test_openInputFile_FailWithMissingFile(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "missing.txt")

	// Act
	file, _, err := openInputFile(path)

	// Assert
	require.Error(t, err)
	assert.Nil(t, file)
	assert.Contains(t, err.Error(), "не удалось проверить файл")
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

// Test_binaryUploadProgressPrinter_SkipsSmallFiles проверяет, что для маленьких файлов прогресс не печатается.
func Test_binaryUploadProgressPrinter_SkipsSmallFiles(t *testing.T) {
	// Arrange
	out := bytes.NewBuffer(nil)
	progress := newBinaryUploadProgressPrinter(out)

	// Act
	progress.update(clientApp.BinaryUploadProgress{
		UploadedBytes: minBinaryUploadProgressSizeBytes - 1,
		TotalBytes:    minBinaryUploadProgressSizeBytes - 1,
	})
	progress.finish()

	// Assert
	assert.Empty(t, out.String())
}

// Test_binaryUploadProgressPrinter_PrintsProgress проверяет вывод прогресса и завершающий перенос строки.
func Test_binaryUploadProgressPrinter_PrintsProgress(t *testing.T) {
	// Arrange
	out := bytes.NewBuffer(nil)
	progress := newBinaryUploadProgressPrinter(out)

	// Act
	progress.update(clientApp.BinaryUploadProgress{
		UploadedBytes: 512 * 1024,
		TotalBytes:    minBinaryUploadProgressSizeBytes,
	})
	progress.finish()

	// Assert
	assert.Equal(t, "\rЗагрузка файла: 0.5 МиБ из 1 МиБ, 50.0%\n", out.String())
}

// Test_binaryUploadProgressPrinter_ClampsUploadedBytes проверяет нормализацию прогресса за границами общего размера.
func Test_binaryUploadProgressPrinter_ClampsUploadedBytes(t *testing.T) {
	// Arrange
	out := bytes.NewBuffer(nil)
	progress := newBinaryUploadProgressPrinter(out)

	// Act
	progress.update(clientApp.BinaryUploadProgress{
		UploadedBytes: 2 * minBinaryUploadProgressSizeBytes,
		TotalBytes:    minBinaryUploadProgressSizeBytes,
	})

	// Assert
	assert.Equal(t, "\rЗагрузка файла: 1 МиБ из 1 МиБ, 100.0%", out.String())
}

// Test_formatMiB проверяет форматирование целых и дробных значений в МиБ.
func Test_formatMiB(t *testing.T) {
	tests := []struct {
		name string
		size int64
		want string
	}{
		{name: "integer", size: 2 * 1024 * 1024, want: "2 МиБ"},
		{name: "fraction", size: 1536 * 1024, want: "1.5 МиБ"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got := formatMiB(tt.size)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}
