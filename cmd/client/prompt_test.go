package main

import (
	"bufio"
	"bytes"
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

// Test_promptRequiredWithError_UsesCustomError проверяет, что ошибка обязательного поля не повторяет текст промпта.
func Test_promptRequiredWithError_UsesCustomError(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("\n"))
	out := bytes.NewBuffer(nil)

	// Act
	_, err := promptRequiredWithError(reader, out, "Выберите действие: ", "действие обязательно")

	// Assert
	require.Error(t, err)
	assert.Equal(t, "действие обязательно", err.Error())
}

// Test_promptRequiredRetry_RepeatsEmptyInput проверяет, что пустой ввод повторяет текущий промпт.
func Test_promptRequiredRetry_RepeatsEmptyInput(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("\nfile.txt\n"))
	out := bytes.NewBuffer(nil)

	// Act
	value, err := promptRequiredRetry(reader, out, "Путь для сохранения файла: ", "путь обязателен")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "file.txt", value)
	assert.Contains(t, out.String(), "ошибка: путь обязателен")
}

// Test_normalizeInput_AppliesBackspaceForASCII проверяет, что управляющий символ Backspace не попадает в команду меню.
func Test_normalizeInput_AppliesBackspaceForASCII(t *testing.T) {
	// Act
	value, err := normalizeInput("13\x7f5\n")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "15", value)
}

// Test_normalizeInput_AppliesBackspaceForCyrillic проверяет удаление предыдущей UTF-8 руны при Backspace.
func Test_normalizeInput_AppliesBackspaceForCyrillic(t *testing.T) {
	// Act
	value, err := normalizeInput("секретыф\x7f\n")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "секреты", value)
}

// Test_normalizeInput_AppliesBackspaceToPartialUTF8 проверяет случай, когда терминал оставил неполную UTF-8
// последовательность перед Backspace.
func Test_normalizeInput_AppliesBackspaceToPartialUTF8(t *testing.T) {
	// Act
	value, err := normalizeInput(string([]byte{'1', 0xd1, 0x7f, '5', '\n'}))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "15", value)
}

// Test_normalizeInput_DropsInvalidUTF8Bytes проверяет, что одиночные битые байты терминального ввода отбрасываются.
func Test_normalizeInput_DropsInvalidUTF8Bytes(t *testing.T) {
	// Act
	value, err := normalizeInput(string([]byte{'a', 0xff, 0xfe, 'b'}))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "ab", value)
}
