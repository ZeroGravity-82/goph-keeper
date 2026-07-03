package main

import (
	"bufio"
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_confirm_AcceptsYOnly проверяет подтверждение только явным ответом "y".
func Test_confirm_AcceptsYOnly(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("Y\n"))
	out := bytes.NewBuffer(nil)

	// Act
	confirmed, err := confirm(reader, out, "Продолжить?")

	// Assert
	require.NoError(t, err)
	assert.True(t, confirmed)
	assert.Equal(t, "Продолжить? [y/N]: ", out.String())
}

// Test_confirm_RejectsEmptyAnswer проверяет значение по умолчанию для подтверждения.
func Test_confirm_RejectsEmptyAnswer(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("\n"))
	out := bytes.NewBuffer(nil)

	// Act
	confirmed, err := confirm(reader, out, "Продолжить?")

	// Assert
	require.NoError(t, err)
	assert.False(t, confirmed)
}

// Test_promptWithDefault_ReturnsCurrentOnEmptyInput проверяет сохранение текущего значения при пустом вводе.
func Test_promptWithDefault_ReturnsCurrentOnEmptyInput(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("\n"))
	out := bytes.NewBuffer(nil)

	// Act
	value, err := promptWithDefault(reader, out, "Название", "Current")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Current", value)
	assert.Equal(t, "Название [Current]: ", out.String())
}

// Test_promptWithDefault_ReturnsInput проверяет замену значения по умолчанию явным вводом.
func Test_promptWithDefault_ReturnsInput(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("New\n"))
	out := bytes.NewBuffer(nil)

	// Act
	value, err := promptWithDefault(reader, out, "Название", "Current")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "New", value)
}

// Test_promptWithDefaultCancelable_Cancel проверяет отмену промпта со значением по умолчанию.
func Test_promptWithDefaultCancelable_Cancel(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("cancel\n"))
	out := bytes.NewBuffer(nil)

	// Act
	_, err := promptWithDefaultCancelable(reader, out, "Название", "Current")

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, errActionCanceled))
}

// Test_printCancelHint проверяет текст подсказки для отмены действия.
func Test_printCancelHint(t *testing.T) {
	// Arrange
	out := bytes.NewBuffer(nil)

	// Act
	printCancelHint(out)

	// Assert
	assert.Equal(t, "Чтобы отменить действие, введите :q, cancel или отмена.\n", out.String())
}

// Test_clearScreen проверяет ANSI-последовательность очистки экрана.
func Test_clearScreen(t *testing.T) {
	// Arrange
	out := bytes.NewBuffer(nil)

	// Act
	clearScreen(out)

	// Assert
	assert.Equal(t, "\033[H\033[2J", out.String())
}

// Test_waitForEnter проверяет ожидание пользовательского Enter.
func Test_waitForEnter(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("\n"))
	out := bytes.NewBuffer(nil)

	// Act
	err := waitForEnter(reader, out)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Нажмите Enter, чтобы продолжить...", out.String())
}

// Test_printActionError_PrintsCanceledWithoutErrorPrefix проверяет пользовательский вывод отмененного действия.
func Test_printActionError_PrintsCanceledWithoutErrorPrefix(t *testing.T) {
	// Arrange
	out := bytes.NewBuffer(nil)

	// Act
	printActionError(out, errActionCanceled)

	// Assert
	assert.Equal(t, "действие отменено\n", out.String())
}

// Test_printActionError_PrintsRegularError проверяет формат обычной ошибки действия.
func Test_printActionError_PrintsRegularError(t *testing.T) {
	// Arrange
	out := bytes.NewBuffer(nil)

	// Act
	printActionError(out, errors.New("нет доступа"))

	// Assert
	assert.Equal(t, "ошибка: нет доступа\n", out.String())
}
