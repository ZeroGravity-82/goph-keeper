package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
)

// Test_printRecordsTable_Empty проверяет вывод списка без записей.
func Test_printRecordsTable_Empty(t *testing.T) {
	// Arrange
	out := bytes.NewBuffer(nil)

	// Act
	printRecordsTable(out, nil)

	// Assert
	assert.Equal(t, "Записи:\n< записей еще нет >\n", out.String())
}

// Test_printRecordsTable_PrintsRowsWithNumbers проверяет вывод строк списка с номерами.
func Test_printRecordsTable_PrintsRowsWithNumbers(t *testing.T) {
	// Arrange
	out := bytes.NewBuffer(nil)
	items := []clientApp.RecordListItem{
		{Type: "text", Title: "Заметка", Description: "Приватная заметка"},
		{Type: "card", Title: "Т-Банк", Description: "Основная карточка от Т-Банка"},
	}

	// Act
	printRecordsTable(out, items)

	// Assert
	assert.Equal(
		t,
		"Записи:\n"+
			"#  Тип   Название  Описание\n"+
			"-  ---   --------  --------\n"+
			"1  text  Заметка   Приватная заметка\n"+
			"2  card  Т-Банк    Основная карточка от Т-Банка\n",
		out.String(),
	)
}

// Test_printRecordsTable_AlignsColumns проверяет выравнивание колонок при разной длине значений.
func Test_printRecordsTable_AlignsColumns(t *testing.T) {
	// Arrange
	out := bytes.NewBuffer(nil)
	items := []clientApp.RecordListItem{
		{Type: "text", Title: "A", Description: "Краткий текст заголовка"},
		{Type: "credential", Title: "Длинный заголовок", Description: "Этот заголовок заметно длиннее"},
	}

	// Act
	printRecordsTable(out, items)

	// Assert
	assert.Equal(
		t,
		"Записи:\n"+
			"#  Тип         Название           Описание\n"+
			"-  ---         --------           --------\n"+
			"1  text        A                  Краткий текст заголовка\n"+
			"2  credential  Длинный заголовок  Этот заголовок заметно длиннее\n",
		out.String(),
	)
}

// Test_printCardRecord_FormatsCardNumber проверяет, что номер карты выводится группами по четыре цифры.
func Test_printCardRecord_FormatsCardNumber(t *testing.T) {
	// Arrange
	out := bytes.NewBuffer(nil)
	record := clientApp.CardRecord{
		Title:       "Т-Банк",
		Description: "Основная карточка от Т-Банка",
		Number:      "4111111111111111",
		HolderName:  "IVAN IVANOV",
		ExpiresAt:   "12/30",
		CVC:         "123",
	}

	// Act
	printCardRecord(out, record)

	// Assert
	assert.Contains(t, out.String(), "* номер карты: 4111 1111 1111 1111\n")
}

// Test_printCredentialRecord проверяет вывод записи с учетными данными.
func Test_printCredentialRecord(t *testing.T) {
	// Arrange
	out := bytes.NewBuffer(nil)
	record := clientApp.CredentialRecord{
		Title:       "Почта",
		Description: "Рабочая почта",
		Login:       "ivan",
		Password:    "secret",
	}

	// Act
	printCredentialRecord(out, record)

	// Assert
	assert.Contains(t, out.String(), "* тип: credential\n")
	assert.Contains(t, out.String(), "* логин: ivan\n")
	assert.Contains(t, out.String(), "* пароль: secret\n")
}

// Test_printBinaryRecord проверяет вывод метаданных бинарной записи.
func Test_printBinaryRecord(t *testing.T) {
	// Arrange
	out := bytes.NewBuffer(nil)
	record := clientApp.BinaryRecord{
		Title:       "Паспорт",
		Description: "Скан",
		Filename:    "passport.pdf",
		ContentType: "application/pdf",
		Size:        42,
	}

	// Act
	printBinaryRecord(out, record)

	// Assert
	assert.Contains(t, out.String(), "* тип: binary\n")
	assert.Contains(t, out.String(), "* исходное имя: passport.pdf\n")
	assert.Contains(t, out.String(), "* размер: 42 байт\n")
}

// Test_formatCardNumber_ReturnsOriginalForInvalidNumber проверяет, что некорректный номер не форматируется.
func Test_formatCardNumber_ReturnsOriginalForInvalidNumber(t *testing.T) {
	// Act
	formatted := formatCardNumber("not-a-card")

	// Assert
	assert.Equal(t, "not-a-card", formatted)
}
