package main

import (
	"bufio"
	"bytes"
	"errors"
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

// Test_promptRequired_ReturnsValue проверяет, что обязательный ввод возвращает непустое значение.
func Test_promptRequired_ReturnsValue(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("value\n"))
	out := bytes.NewBuffer(nil)

	// Act
	value, err := promptRequired(reader, out, "Поле: ")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "value", value)
	assert.Equal(t, "Поле: ", out.String())
}

// Test_promptRequiredNamed_UsesFieldName проверяет сообщение об ошибке для именованного обязательного поля.
func Test_promptRequiredNamed_UsesFieldName(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("\n"))
	out := bytes.NewBuffer(nil)

	// Act
	_, err := promptRequiredNamed(reader, out, "Логин: ", "логин")

	// Assert
	require.Error(t, err)
	assert.Equal(t, "логин обязателен", err.Error())
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

// Test_promptRequiredRetryCancelable_ReturnsCancel проверяет, что обязательный промпт с повтором ввода завершает
// действие по команде отмены и не печатает ее как ошибку валидации.
func Test_promptRequiredRetryCancelable_ReturnsCancel(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString(":q\n"))
	out := bytes.NewBuffer(nil)

	// Act
	value, err := promptRequiredRetryCancelable(reader, out, "Название: ", "название обязательно")

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, errActionCanceled))
	assert.Empty(t, value)
	assert.NotContains(t, out.String(), "ошибка:")
}

// Test_promptRequiredCancelable_ReturnsValue проверяет обязательный отменяемый ввод без повтора.
func Test_promptRequiredCancelable_ReturnsValue(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("Название\n"))
	out := bytes.NewBuffer(nil)

	// Act
	value, err := promptRequiredCancelable(reader, out, "Название: ")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Название", value)
}

// Test_promptCancelable_AcceptsRussianCancel проверяет, что обычный отменяемый промпт принимает команду отмены на
// русском языке.
func Test_promptCancelable_AcceptsRussianCancel(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("отмена\n"))
	out := bytes.NewBuffer(nil)

	// Act
	_, err := promptCancelable(reader, out, "Описание: ")

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, errActionCanceled))
}

// Test_promptSecretConfirmedRequired_ReturnsRequiredErrorBeforeConfirmation проверяет, что пустой первый ввод секрета
// не приводит к запросу подтверждения.
func Test_promptSecretConfirmedRequired_ReturnsRequiredErrorBeforeConfirmation(t *testing.T) {
	// Arrange
	input := bytes.NewBufferString("\nsecret\n")
	reader := bufio.NewReader(input)
	out := bytes.NewBuffer(nil)

	// Act
	_, err := promptSecretConfirmedRequired(
		reader,
		input,
		out,
		"Пароль: ",
		"Повторите пароль: ",
		"пароль обязателен",
	)

	// Assert
	require.Error(t, err)
	assert.Equal(t, "пароль обязателен", err.Error())
	assert.Equal(t, "Пароль: ", out.String())
}

// Test_promptSecretConfirmed_ReturnsValue проверяет успешный повторный ввод секретного значения.
func Test_promptSecretConfirmed_ReturnsValue(t *testing.T) {
	// Arrange
	input := bytes.NewBufferString("secret\nsecret\n")
	reader := bufio.NewReader(input)
	out := bytes.NewBuffer(nil)

	// Act
	value, err := promptSecretConfirmed(reader, input, out, "Пароль: ", "Повторите пароль: ")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "secret", value)
}

// Test_promptSecretConfirmed_RejectsMismatch проверяет ошибку при несовпадении секретных значений.
func Test_promptSecretConfirmed_RejectsMismatch(t *testing.T) {
	// Arrange
	input := bytes.NewBufferString("secret\nother\n")
	reader := bufio.NewReader(input)
	out := bytes.NewBuffer(nil)

	// Act
	_, err := promptSecretConfirmed(reader, input, out, "Пароль: ", "Повторите пароль: ")

	// Assert
	require.Error(t, err)
	assert.Equal(t, "значения не совпадают", err.Error())
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

// Test_normalizeInput_SkipsEscapeSequence проверяет, что escape-последовательность стрелки не попадает во ввод.
func Test_normalizeInput_SkipsEscapeSequence(t *testing.T) {
	// Act
	value, err := normalizeInput("1\x1b[A2")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "12", value)
}
