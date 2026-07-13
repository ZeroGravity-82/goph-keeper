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
	printRecordsMenu(out, items, false)

	// Assert
	assert.Contains(t, out.String(), "Записи:\n")
	assert.Contains(t, out.String(), "1  text  Моя заметка  Совершенно секретно!\n")
	assert.Contains(t, out.String(), "Действия:\n")
	assert.Contains(t, out.String(), "7. Сменить мастер-ключ\n")
	assert.Contains(t, out.String(), "9. Завершить приложение\n")
}

// Test_printRecordsMenu_Readonly проверяет вывод статуса режима чтения.
func Test_printRecordsMenu_Readonly(t *testing.T) {
	// Arrange
	out := bytes.NewBuffer(nil)

	// Act
	printRecordsMenu(out, nil, true)

	// Assert
	assert.Contains(t, out.String(), "Режим чтения: создание, изменение и удаление записей временно недоступны.\n")
}

// Test_handleConnectionError_ReadonlyPrintsReminder проверяет сообщение о том, что режим чтения все еще активен.
func Test_handleConnectionError_ReadonlyPrintsReminder(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	state.readonly = true
	out := bytes.NewBuffer(nil)

	// Act
	handleConnectionError(state, out, clientApp.ErrServerUnavailable)

	// Assert
	assert.True(t, state.readonly)
	assert.Equal(t, "режим чтения: связь с сервером все еще недоступна\n", out.String())
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

// Test_openSelectedRecord_ReadonlyUsesCache проверяет открытие уже загруженной записи без обращения к серверу в режиме
// чтения.
func Test_openSelectedRecord_ReadonlyUsesCache(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	state.readonly = true
	state.storeText(clientApp.TextRecord{
		RecordID:    "record-1",
		Title:       "Заметка",
		Description: "Кешированная",
		Text:        "секрет",
	})
	item := clientApp.RecordListItem{RecordID: "record-1", Type: "text"}
	out := bytes.NewBuffer(nil)

	// Act
	err := openSelectedRecord(context.Background(), nil, state, item, out)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, out.String(), "* текст: секрет\n")
}

// Test_openSelectedRecord_ReadonlyFailsWithoutCachedRecord проверяет ошибку, если запись не была загружена до перехода
// в режим чтения.
func Test_openSelectedRecord_ReadonlyFailsWithoutCachedRecord(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	state.readonly = true
	item := clientApp.RecordListItem{RecordID: "record-1", Type: "text"}
	out := bytes.NewBuffer(nil)

	// Act
	err := openSelectedRecord(context.Background(), nil, state, item, out)

	// Assert
	require.Error(t, err)
	assert.Equal(t, "запись не загружена за текущий запуск приложения", err.Error())
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
