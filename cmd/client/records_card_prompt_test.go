package main

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_promptCardNumber_RetriesInvalidInput проверяет повторный ввод номера карты после ошибки валидации.
func Test_promptCardNumber_RetriesInvalidInput(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("1234\n4111 1111 1111 1111\n"))
	out := bytes.NewBuffer(nil)

	// Act
	number, err := promptCardNumber(reader, out, "Номер карты: ")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "4111111111111111", number)
	assert.Contains(t, out.String(), "ошибка: номер карты должен содержать 16 цифр")
}

// Test_promptCardNumber_RetriesInvalidChecksum проверяет повторный ввод номера карты после ошибки контрольной суммы.
func Test_promptCardNumber_RetriesInvalidChecksum(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("4111 1111 1111 1112\n4111 1111 1111 1111\n"))
	out := bytes.NewBuffer(nil)

	// Act
	number, err := promptCardNumber(reader, out, "Номер карты: ")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "4111111111111111", number)
	assert.Contains(t, out.String(), "ошибка: неверный номер карты")
}

// Test_promptCardNumberWithDefault_ReturnsCurrentOnEmptyInput проверяет значение по умолчанию для номера карты.
func Test_promptCardNumberWithDefault_ReturnsCurrentOnEmptyInput(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("\n"))
	out := bytes.NewBuffer(nil)

	// Act
	number, err := promptCardNumberWithDefault(reader, out, "Новый номер карты", "4111111111111111")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "4111111111111111", number)
	assert.Contains(t, out.String(), "Новый номер карты [4111 1111 1111 1111]: ")
}

// Test_promptCardHolderName_NormalizesValue проверяет нормализацию имени владельца карты.
func Test_promptCardHolderName_NormalizesValue(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("ivan ivanov\n"))
	out := bytes.NewBuffer(nil)

	// Act
	holderName, err := promptCardHolderName(reader, out, "Имя владельца: ")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "IVAN IVANOV", holderName)
}

// Test_promptCardHolderName_RetriesCyrillicInput проверяет повторный ввод имени владельца после кириллицы.
func Test_promptCardHolderName_RetriesCyrillicInput(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("ИВАН ИВАНОВ\nivan ivanov\n"))
	out := bytes.NewBuffer(nil)

	// Act
	holderName, err := promptCardHolderName(reader, out, "Имя владельца: ")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "IVAN IVANOV", holderName)
	assert.Contains(t, out.String(), "ошибка: имя владельца карты должно содержать только латинские буквы и пробелы")
}

// Test_promptCardExpiration_RetriesInvalidInput проверяет повторный ввод срока действия после ошибки формата.
func Test_promptCardExpiration_RetriesInvalidInput(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("13/30\n12/30\n"))
	out := bytes.NewBuffer(nil)

	// Act
	expiresAt, err := promptCardExpiration(reader, out, "Срок действия: ")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "12/30", expiresAt)
	assert.Contains(t, out.String(), "ошибка: срок действия карты должен быть в формате ММ/ГГ")
}

// Test_promptCardCVC_RetriesInvalidInput проверяет повторный ввод CVC после ошибки формата.
func Test_promptCardCVC_RetriesInvalidInput(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("12\n123\n"))
	out := bytes.NewBuffer(nil)

	// Act
	cvc, err := promptCardCVC(reader, out, "CVC: ")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "123", cvc)
	assert.Contains(t, out.String(), "ошибка: CVC должен содержать ровно 3 цифры")
}
