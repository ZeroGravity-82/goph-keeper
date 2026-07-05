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

// Test_createCredential_CancelBeforeSave проверяет отмену создания записи с учетными данными до обращения к клиенту.
func Test_createCredential_CancelBeforeSave(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("Mail\nWork\nuser\n:q\n"))
	out := bytes.NewBuffer(nil)

	// Act
	err := createCredential(context.Background(), nil, reader, out)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, errActionCanceled))
	assert.Contains(t, out.String(), "Пароль: ")
}

// Test_createText_CancelBeforeSave проверяет отмену создания текстовой записи до обращения к клиенту.
func Test_createText_CancelBeforeSave(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("Note\nPrivate\n:q\n"))
	out := bytes.NewBuffer(nil)

	// Act
	err := createText(context.Background(), nil, reader, out)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, errActionCanceled))
	assert.Contains(t, out.String(), "Текст: ")
}

// Test_createCard_CancelBeforeSave проверяет отмену создания банковской карты после ввода валидных полей.
func Test_createCard_CancelBeforeSave(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("Card\nMain\n4111 1111 1111 1111\nJOHN DOE\n12/30\n:q\n"))
	out := bytes.NewBuffer(nil)

	// Act
	err := createCard(context.Background(), nil, reader, out)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, errActionCanceled))
	assert.Contains(t, out.String(), "CVC: ")
}

// Test_createBinary_CancelBeforeReadFile проверяет отмену создания файловой записи до чтения файла.
func Test_createBinary_CancelBeforeReadFile(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString("Doc\nPrivate\n:q\n"))
	out := bytes.NewBuffer(nil)

	// Act
	err := createBinary(context.Background(), nil, reader, out)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, errActionCanceled))
	assert.Contains(t, out.String(), "Путь к файлу: ")
}

// Test_updateSelectedCredential_CancelBeforeSave проверяет отмену изменения учетных данных после чтения записи из кеша.
func Test_updateSelectedCredential_CancelBeforeSave(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	record := clientApp.CredentialRecord{
		RecordID:    "019539de-3dfd-76b6-914a-b9fef732ece9",
		Version:     2,
		Title:       "Mail",
		Description: "Work",
		Login:       "user",
		Password:    "secret",
	}
	state.storeCredential(record)
	reader := bufio.NewReader(bytes.NewBufferString("\n\n\n:q\n"))
	out := bytes.NewBuffer(nil)

	// Act
	err := updateSelectedCredential(
		context.Background(),
		nil,
		state,
		clientApp.RecordListItem{RecordID: record.RecordID},
		reader,
		out,
	)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, errActionCanceled))
	assert.Contains(t, out.String(), "Новый пароль")
}

// Test_updateSelectedText_CancelBeforeSave проверяет отмену изменения текстовой записи после чтения записи из кеша.
func Test_updateSelectedText_CancelBeforeSave(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	record := clientApp.TextRecord{
		RecordID:    "019539de-5820-7a98-b07e-42d049b6d6c5",
		Version:     2,
		Title:       "Note",
		Description: "Private",
		Text:        "secret",
	}
	state.storeText(record)
	reader := bufio.NewReader(bytes.NewBufferString("\n\n:q\n"))
	out := bytes.NewBuffer(nil)

	// Act
	err := updateSelectedText(
		context.Background(),
		nil,
		state,
		clientApp.RecordListItem{RecordID: record.RecordID},
		reader,
		out,
	)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, errActionCanceled))
	assert.Contains(t, out.String(), "Новый текст")
}

// Test_updateSelectedCard_CancelBeforeSave проверяет отмену изменения банковской карты после чтения записи из кеша.
func Test_updateSelectedCard_CancelBeforeSave(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	record := clientApp.CardRecord{
		RecordID:    "019539de-6a64-7991-a160-79d193d7dc85",
		Version:     2,
		Title:       "Card",
		Description: "Main",
		Number:      "4111111111111111",
		HolderName:  "JOHN DOE",
		ExpiresAt:   "12/30",
		CVC:         "123",
	}
	state.storeCard(record)
	reader := bufio.NewReader(bytes.NewBufferString("\n\n\n\n\n:q\n"))
	out := bytes.NewBuffer(nil)

	// Act
	err := updateSelectedCard(
		context.Background(),
		nil,
		state,
		clientApp.RecordListItem{RecordID: record.RecordID},
		reader,
		out,
	)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, errActionCanceled))
	assert.Contains(t, out.String(), "Новый CVC")
}

// Test_updateSelectedBinaryMetadata_CancelBeforeSave проверяет отмену изменения метаданных файла после чтения кеша.
func Test_updateSelectedBinaryMetadata_CancelBeforeSave(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	record := clientApp.BinaryRecord{
		RecordID:    "019539de-7bbc-7fe1-88ef-2760a320b31c",
		Version:     2,
		Title:       "Doc",
		Description: "Private",
		Filename:    "doc.txt",
	}
	state.storeBinary(record)
	reader := bufio.NewReader(bytes.NewBufferString("\n:q\n"))
	out := bytes.NewBuffer(nil)

	// Act
	err := updateSelectedBinaryMetadata(
		context.Background(),
		nil,
		state,
		clientApp.RecordListItem{RecordID: record.RecordID},
		reader,
		out,
	)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, errActionCanceled))
	assert.Contains(t, out.String(), "Новое описание")
}

// Test_replaceSelectedBinaryFile_CancelBeforeReadFile проверяет отмену замены файла до чтения нового файла.
func Test_replaceSelectedBinaryFile_CancelBeforeReadFile(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	record := clientApp.BinaryRecord{
		RecordID:    "019539de-8d45-7741-8a70-78b94c35ee27",
		Version:     2,
		Title:       "Doc",
		Description: "Private",
		Filename:    "doc.txt",
	}
	state.storeBinary(record)
	reader := bufio.NewReader(bytes.NewBufferString("\n\n:q\n"))
	out := bytes.NewBuffer(nil)

	// Act
	err := replaceSelectedBinaryFile(
		context.Background(),
		nil,
		state,
		clientApp.RecordListItem{RecordID: record.RecordID},
		reader,
		out,
	)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, errActionCanceled))
	assert.Contains(t, out.String(), "Путь к новому файлу: ")
}

// Test_downloadSelectedBinaryFile_CancelBeforeDownload проверяет отмену скачивания до обращения к клиенту.
func Test_downloadSelectedBinaryFile_CancelBeforeDownload(t *testing.T) {
	// Arrange
	reader := bufio.NewReader(bytes.NewBufferString(":q\n"))
	out := bytes.NewBuffer(nil)

	// Act
	err := downloadSelectedBinaryFile(
		context.Background(),
		nil,
		clientApp.RecordListItem{RecordID: "019539de-a1f4-7d3e-a66d-2ca19d3c5a18"},
		reader,
		out,
	)

	// Assert
	require.Error(t, err)
	assert.True(t, errors.Is(err, errActionCanceled))
	assert.Contains(t, out.String(), "Директория для сохранения файла: ")
}

// Test_deleteSelectedRecord_CancelKeepsState проверяет отказ от удаления выбранной записи без обращения к клиенту.
func Test_deleteSelectedRecord_CancelKeepsState(t *testing.T) {
	// Arrange
	state := newRecordsMenuState()
	item := clientApp.RecordListItem{
		RecordID: "019539de-b728-7947-a78f-56fc09b308d1",
		Type:     "text",
		Title:    "Note",
	}
	state.storeItem(item)
	reader := bufio.NewReader(bytes.NewBufferString("n\n"))
	out := bytes.NewBuffer(nil)

	// Act
	err := deleteSelectedRecord(context.Background(), nil, state, item, reader, out)

	// Assert
	require.NoError(t, err)
	require.Len(t, state.items, 1)
	assert.Contains(t, out.String(), "удаление отменено")
}
