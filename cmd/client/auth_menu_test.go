package main

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
	"zerogravity-82/goph-keeper/internal/buildinfo"
)

// Test_printStartMenu проверяет пункты стартового меню.
func Test_printStartMenu(t *testing.T) {
	// Arrange
	out := bytes.NewBuffer(nil)

	// Act
	printStartMenu(out, buildinfo.New("1.2.3", "2026-07-05"))

	// Assert
	assert.Equal(
		t,
		"GophKeeper (версия: 1.2.3, дата сборки: 2026-07-05)\n\n"+
			"1. Зарегистрироваться\n"+
			"2. Войти в аккаунт\n"+
			"3. Завершить приложение\n\n",
		out.String(),
	)
}

// Test_printWelcome проверяет приветствие после входа.
func Test_printWelcome(t *testing.T) {
	// Arrange
	out := bytes.NewBuffer(nil)

	// Act
	printWelcome(out, "ivan")

	// Assert
	assert.Equal(t, "Добро пожаловать, ivan.\n", out.String())
}

// Test_prepareMasterKey_ReturnsEnteredMasterKey проверяет получение мастер-ключа при наличии проверочных данных.
func Test_prepareMasterKey_ReturnsEnteredMasterKey(t *testing.T) {
	// Arrange
	input := bytes.NewBufferString("secret\n")
	reader := bufio.NewReader(input)
	out := bytes.NewBuffer(nil)
	session := clientApp.AuthSession{MasterKeyVerifier: []byte("verifier")}

	// Act
	gotSession, masterKey, err := prepareMasterKey(session, reader, input, out)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, session, gotSession)
	assert.Equal(t, "secret", masterKey)
}

// Test_prepareMasterKey_RequiresVerifier проверяет ошибку при отсутствии проверочных данных мастер-ключа.
func Test_prepareMasterKey_RequiresVerifier(t *testing.T) {
	// Arrange
	input := bytes.NewBufferString("secret\n")
	reader := bufio.NewReader(input)
	out := bytes.NewBuffer(nil)

	// Act
	_, _, err := prepareMasterKey(clientApp.AuthSession{}, reader, input, out)

	// Assert
	require.Error(t, err)
	assert.Equal(t, "проверочные данные мастер-ключа отсутствуют", err.Error())
}

// Test_promptMasterKey_ReturnsValue проверяет ввод непустого мастер-ключа.
func Test_promptMasterKey_ReturnsValue(t *testing.T) {
	// Arrange
	input := bytes.NewBufferString("secret\n")
	reader := bufio.NewReader(input)
	out := bytes.NewBuffer(nil)

	// Act
	value, err := promptMasterKey(reader, input, out)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "secret", value)
}

// Test_promptMasterKey_RequiresValue проверяет ошибку для пустого мастер-ключа.
func Test_promptMasterKey_RequiresValue(t *testing.T) {
	// Arrange
	input := bytes.NewBufferString("\n")
	reader := bufio.NewReader(input)
	out := bytes.NewBuffer(nil)

	// Act
	_, err := promptMasterKey(reader, input, out)

	// Assert
	require.Error(t, err)
	assert.Equal(t, "мастер-ключ обязателен", err.Error())
}

// Test_promptMasterKeyConfirmed_ReturnsValue проверяет успешный повторный ввод мастер-ключа.
func Test_promptMasterKeyConfirmed_ReturnsValue(t *testing.T) {
	// Arrange
	input := bytes.NewBufferString("secret\nsecret\n")
	reader := bufio.NewReader(input)
	out := bytes.NewBuffer(nil)

	// Act
	value, err := promptMasterKeyConfirmed(reader, input, out)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "secret", value)
}

// Test_promptMasterKeyConfirmed_RejectsMismatch проверяет ошибку при несовпадении мастер-ключей.
func Test_promptMasterKeyConfirmed_RejectsMismatch(t *testing.T) {
	// Arrange
	input := bytes.NewBufferString("secret\nother\n")
	reader := bufio.NewReader(input)
	out := bytes.NewBuffer(nil)

	// Act
	_, err := promptMasterKeyConfirmed(reader, input, out)

	// Assert
	require.Error(t, err)
	assert.Equal(t, "значения не совпадают", err.Error())
}

// Test_promptMasterKeyConfirmed_RequiresFirstValue проверяет, что пустой мастер-ключ отклоняется до повторного ввода.
func Test_promptMasterKeyConfirmed_RequiresFirstValue(t *testing.T) {
	// Arrange
	input := bytes.NewBufferString("\nsecret\n")
	reader := bufio.NewReader(input)
	out := bytes.NewBuffer(nil)

	// Act
	_, err := promptMasterKeyConfirmed(reader, input, out)

	// Assert
	require.Error(t, err)
	assert.Equal(t, "мастер-ключ обязателен", err.Error())
	assert.Equal(t, "Мастер-ключ: ", out.String())
}
