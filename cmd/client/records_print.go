package main

import (
	"fmt"
	"io"
	"text/tabwriter"

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
)

// printRecordsTable печатает текущий список записей в табличном виде.
func printRecordsTable(out io.Writer, items []clientApp.RecordListItem) {
	fmt.Fprintln(out, "Записи:")
	if len(items) == 0 {
		fmt.Fprintln(out, "< записей еще нет >")
		return
	}
	table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "#\tТип\tНазвание\tОписание")
	fmt.Fprintln(table, "-\t---\t--------\t--------")
	for i, item := range items {
		fmt.Fprintf(table, "%d\t%s\t%s\t%s\n", i+1, item.Type, item.Title, item.Description)
	}
	_ = table.Flush()
}

// printCredentialRecord печатает расшифрованную запись с учетными данными.
func printCredentialRecord(out io.Writer, record clientApp.CredentialRecord) {
	fmt.Fprintf(out, "* тип: credential\n")
	fmt.Fprintf(out, "* название: %s\n", record.Title)
	fmt.Fprintf(out, "* описание: %s\n", record.Description)
	fmt.Fprintf(out, "* логин сохраненной записи с учетными данными: %s\n", record.Login)
	fmt.Fprintf(out, "* пароль сохраненной записи с учетными данными: %s\n", record.Password)
}

// printTextRecord печатает расшифрованную текстовую запись.
func printTextRecord(out io.Writer, record clientApp.TextRecord) {
	fmt.Fprintf(out, "* тип: text\n")
	fmt.Fprintf(out, "* название: %s\n", record.Title)
	fmt.Fprintf(out, "* описание: %s\n", record.Description)
	fmt.Fprintf(out, "* текст: %s\n", record.Text)
}

// printCardRecord печатает расшифрованную запись банковской карты.
func printCardRecord(out io.Writer, record clientApp.CardRecord) {
	fmt.Fprintf(out, "* тип: card\n")
	fmt.Fprintf(out, "* название: %s\n", record.Title)
	fmt.Fprintf(out, "* описание: %s\n", record.Description)
	fmt.Fprintf(out, "* номер карты: %s\n", formatCardNumber(record.Number))
	fmt.Fprintf(out, "* имя владельца: %s\n", record.HolderName)
	fmt.Fprintf(out, "* срок действия: %s\n", record.ExpiresAt)
	fmt.Fprintf(out, "* CVC: %s\n", record.CVC)
}

// printBinaryRecord печатает метаданные бинарной записи.
func printBinaryRecord(out io.Writer, record clientApp.BinaryRecord) {
	fmt.Fprintf(out, "* тип: binary\n")
	fmt.Fprintf(out, "* название: %s\n", record.Title)
	fmt.Fprintf(out, "* описание: %s\n", record.Description)
	fmt.Fprintf(out, "* исходное имя: %s\n", record.Filename)
	fmt.Fprintf(out, "* MIME-тип: %s\n", record.ContentType)
	fmt.Fprintf(out, "* размер: %d байт\n", record.Size)
}

// formatCardNumber форматирует номер карты группами по четыре цифры.
func formatCardNumber(number string) string {
	normalized, err := clientApp.NormalizeCardNumber(number)
	if err != nil || len(normalized) != 16 {
		return number
	}
	return normalized[:4] + " " + normalized[4:8] + " " + normalized[8:12] + " " + normalized[12:16]
}
