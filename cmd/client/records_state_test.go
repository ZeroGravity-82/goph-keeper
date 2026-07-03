package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
)

// Test_newRecordsMenuState_CreatesEmptyCache проверяет начальное состояние меню записей.
func Test_newRecordsMenuState_CreatesEmptyCache(t *testing.T) {
	// Act
	state := newRecordsMenuState()

	// Assert
	require.NotNil(t, state.cache)
	assert.Empty(t, state.items)
	assert.Empty(t, state.cache)
}

// Test_recordsMenuState_storeItem_UpdatesExistingItem проверяет, что повторное сохранение записи не дублирует строку.
func Test_recordsMenuState_storeItem_UpdatesExistingItem(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	state.storeItem(clientApp.RecordListItem{
		RecordID:    "record-1",
		Type:        "text",
		Title:       "Старый",
		Description: "Старое описание",
	})

	// Act
	state.storeItem(clientApp.RecordListItem{
		RecordID:    "record-1",
		Type:        "text",
		Title:       "Новый",
		Description: "Новое описание",
	})

	// Assert
	require.Len(t, state.items, 1)
	assert.Equal(t, "Новый", state.items[0].Title)
	assert.Equal(t, "Новое описание", state.items[0].Description)
}

// Test_recordsMenuState_storeCredential_CachesFullRecord проверяет кеширование полной записи с учетными данными.
func Test_recordsMenuState_storeCredential_CachesFullRecord(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	record := clientApp.CredentialRecord{
		RecordID:    "record-1",
		Version:     3,
		Title:       "Mail",
		Description: "Work mailbox",
		Login:       "user",
		Password:    "secret",
	}

	// Act
	state.storeCredential(record)

	// Assert
	require.Len(t, state.items, 1)
	assert.Equal(t, "credential", state.items[0].Type)
	assert.Equal(t, "Mail", state.items[0].Title)
	require.NotNil(t, state.cache["record-1"].credential)
	assert.Equal(t, int64(3), state.cache["record-1"].credential.Version)
}

// Test_recordsMenuState_storeText_CachesFullRecord проверяет кеширование полной текстовой записи.
func Test_recordsMenuState_storeText_CachesFullRecord(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	record := clientApp.TextRecord{
		RecordID:    "record-1",
		Version:     2,
		Title:       "Note",
		Description: "Private",
		Text:        "secret",
	}

	// Act
	state.storeText(record)

	// Assert
	require.Len(t, state.items, 1)
	assert.Equal(t, "text", state.items[0].Type)
	require.NotNil(t, state.cache["record-1"].text)
	assert.Equal(t, "secret", state.cache["record-1"].text.Text)
}

// Test_recordsMenuState_storeCard_CachesFullRecord проверяет кеширование полной записи банковской карты.
func Test_recordsMenuState_storeCard_CachesFullRecord(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	record := clientApp.CardRecord{
		RecordID:    "record-1",
		Version:     2,
		Title:       "Card",
		Description: "Main",
		Number:      "4111111111111111",
	}

	// Act
	state.storeCard(record)

	// Assert
	require.Len(t, state.items, 1)
	assert.Equal(t, "card", state.items[0].Type)
	require.NotNil(t, state.cache["record-1"].card)
	assert.Equal(t, "4111111111111111", state.cache["record-1"].card.Number)
}

// Test_recordsMenuState_storeBinary_CachesFullRecord проверяет кеширование полной бинарной записи.
func Test_recordsMenuState_storeBinary_CachesFullRecord(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	record := clientApp.BinaryRecord{
		RecordID:    "record-1",
		Version:     2,
		Title:       "File",
		Description: "Doc",
		Filename:    "doc.pdf",
	}

	// Act
	state.storeBinary(record)

	// Assert
	require.Len(t, state.items, 1)
	assert.Equal(t, "binary", state.items[0].Type)
	require.NotNil(t, state.cache["record-1"].binary)
	assert.Equal(t, "doc.pdf", state.cache["record-1"].binary.Filename)
}

// Test_cachedCredential_ReturnsCachedRecord проверяет получение записи с учетными данными из кеша без обращения к
// серверу.
func Test_cachedCredential_ReturnsCachedRecord(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	state.storeCredential(clientApp.CredentialRecord{RecordID: "record-1", Version: 1, Title: "Mail"})
	item := clientApp.RecordListItem{RecordID: "record-1"}

	// Act
	record, err := cachedCredential(context.Background(), nil, state, item)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Mail", record.Title)
}

// Test_cachedText_ReturnsCachedRecord проверяет получение текстовой записи из кеша без обращения к серверу.
func Test_cachedText_ReturnsCachedRecord(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	state.storeText(clientApp.TextRecord{RecordID: "record-1", Version: 1, Title: "Note"})
	item := clientApp.RecordListItem{RecordID: "record-1"}

	// Act
	record, err := cachedText(context.Background(), nil, state, item)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Note", record.Title)
}

// Test_cachedCard_ReturnsCachedRecord проверяет получение записи банковской карты из кеша без обращения к серверу.
func Test_cachedCard_ReturnsCachedRecord(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	state.storeCard(clientApp.CardRecord{RecordID: "record-1", Version: 1, Title: "Card"})
	item := clientApp.RecordListItem{RecordID: "record-1"}

	// Act
	record, err := cachedCard(context.Background(), nil, state, item)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Card", record.Title)
}

// Test_cachedBinary_ReturnsCachedRecord проверяет получение бинарной записи из кеша без обращения к серверу.
func Test_cachedBinary_ReturnsCachedRecord(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	state.storeBinary(clientApp.BinaryRecord{RecordID: "record-1", Version: 1, Title: "File"})
	item := clientApp.RecordListItem{RecordID: "record-1"}

	// Act
	record, err := cachedBinary(context.Background(), nil, state, item)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "File", record.Title)
}
