package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
)

// Test_promptRecordRow_Valid проверяет выбор строки записи по номеру.
func Test_promptRecordRow_Valid(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("2\n"))
	out := bytes.NewBuffer(nil)

	// Act
	row, err := promptRecordRow(reader, out, 3)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2, row)
}

// Test_promptRecordRow_Cancel проверяет отмену выбора строки записи.
func Test_promptRecordRow_Cancel(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString(":q\n"))
	out := bytes.NewBuffer(nil)

	// Act
	_, err := promptRecordRow(reader, out, 3)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, errActionCanceled))
}

// Test_promptRecordRow_OutOfRange проверяет ошибку для номера вне диапазона.
func Test_promptRecordRow_OutOfRange(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("4\n"))
	out := bytes.NewBuffer(nil)

	// Act
	_, err := promptRecordRow(reader, out, 3)

	// Assert
	require.Error(t, err)
	assert.Equal(t, "номер записи должен быть от 1 до 3", err.Error())
}

// Test_promptRecordRow_OutOfRangeForSingleRecord проверяет ошибку выбора при единственной записи.
func Test_promptRecordRow_OutOfRangeForSingleRecord(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("2\n"))
	out := bytes.NewBuffer(nil)

	// Act
	_, err := promptRecordRow(reader, out, 1)

	// Assert
	require.Error(t, err)
	assert.Equal(t, "номер записи должен быть 1", err.Error())
}

// Test_printRecordActionMenu_Binary проверяет, что для бинарной записи доступны файловые действия.
func Test_printRecordActionMenu_Binary(t *testing.T) {
	// Arrange
	out := bytes.NewBuffer(nil)

	// Act
	printRecordActionMenu(out, "binary")

	// Assert
	assert.Contains(t, out.String(), "3. Скачать файл\n")
	assert.Contains(t, out.String(), "4. Заменить файл\n")
}

// Test_printRecordActionMenu_Text проверяет, что для обычной записи недоступны файловые действия.
func Test_printRecordActionMenu_Text(t *testing.T) {
	// Arrange
	out := bytes.NewBuffer(nil)

	// Act
	printRecordActionMenu(out, "text")

	// Assert
	assert.Contains(t, out.String(), "1. Изменить\n")
	assert.NotContains(t, out.String(), "Скачать файл")
	assert.NotContains(t, out.String(), "Заменить файл")
}

// Test_printRecordsMenu проверяет вывод списка записей вместе с меню действий.
func Test_printRecordsMenu(t *testing.T) {
	// Arrange
	out := bytes.NewBuffer(nil)
	items := []clientApp.RecordListItem{{Type: "text", Title: "Моя заметка", Description: "Совершенно секретно!"}}

	// Act
	printRecordsMenu(out, items)

	// Assert
	assert.Contains(t, out.String(), "Записи:\n")
	assert.Contains(t, out.String(), "1  text  Моя заметка  Совершенно секретно!\n")
	assert.Contains(t, out.String(), "Действия:\n")
	assert.Contains(t, out.String(), "8. Завершить приложение\n")
}

// Test_operateSelectedRecord_EmptyList проверяет ошибку выбора записи из пустого списка.
func Test_operateSelectedRecord_EmptyList(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	reader := bufio.NewReader(bytes.NewBufferString(""))
	out := bytes.NewBuffer(nil)

	// Act
	err := operateSelectedRecord(context.Background(), nil, state, reader, out)

	// Assert
	require.Error(t, err)
	assert.Equal(t, "список приватных записей пуст", err.Error())
}

// Test_openSelectedRecord_UnknownType проверяет ошибку для неизвестного типа записи.
func Test_openSelectedRecord_UnknownType(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	item := clientApp.RecordListItem{RecordID: "record-1", Type: "unknown"}
	out := bytes.NewBuffer(nil)

	// Act
	err := openSelectedRecord(context.Background(), nil, state, item, out)

	// Assert
	require.Error(t, err)
	assert.Equal(t, "неподдерживаемый тип приватной записи: unknown", err.Error())
}

// Test_updateSelectedRecord_UnknownType проверяет ошибку обновления записи неизвестного типа.
func Test_updateSelectedRecord_UnknownType(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	item := clientApp.RecordListItem{RecordID: "record-1", Type: "unknown"}
	reader := bufio.NewReader(bytes.NewBufferString(""))
	out := bytes.NewBuffer(nil)

	// Act
	err := updateSelectedRecord(context.Background(), nil, state, item, reader, out)

	// Assert
	require.Error(t, err)
	assert.Equal(t, "неподдерживаемый тип приватной записи: unknown", err.Error())
}
