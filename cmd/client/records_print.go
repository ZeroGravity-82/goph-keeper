package main

import (
	"fmt"
	"io"
	"text/tabwriter"

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
)

// printRecordsTable печатает текущий список записей в табличном виде.
func printRecordsTable(out io.Writer, items []clientApp.RecordListItem) {
	_, _ = fmt.Fprintln(out, "Записи:")
	if len(items) == 0 {
		_, _ = fmt.Fprintln(out, "[записей еще нет]")
		return
	}
	table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if hasUploadStatus(items) {
		_, _ = fmt.Fprintln(table, "#\tТип\tНазвание\tОписание\tСтатус файла")
		_, _ = fmt.Fprintln(table, "-\t---\t--------\t--------\t------------")
		for i, item := range items {
			_, _ = fmt.Fprintf(
				table,
				"%d\t%s\t%s\t%s\t%s\n",
				i+1,
				item.Type,
				item.Title,
				item.Description,
				uploadStatusForTable(item),
			)
		}
	} else {
		_, _ = fmt.Fprintln(table, "#\tТип\tНазвание\tОписание")
		_, _ = fmt.Fprintln(table, "-\t---\t--------\t--------")
		for i, item := range items {
			_, _ = fmt.Fprintf(table, "%d\t%s\t%s\t%s\n", i+1, item.Type, item.Title, item.Description)
		}
	}
	_ = table.Flush()
}

// hasUploadStatus проверяет, нужно ли выводить колонку статуса файла в списке записей.
func hasUploadStatus(items []clientApp.RecordListItem) bool {
	for _, item := range items {
		if item.UploadStatus != "" {
			return true
		}
	}
	return false
}

// uploadStatusForTable возвращает статус загрузки файла для таблицы или прочерк для записей без файла.
func uploadStatusForTable(item clientApp.RecordListItem) string {
	if item.UploadStatus == "" {
		return "-"
	}
	return string(item.UploadStatus)
}

// printCredentialRecord печатает расшифрованную запись с учетными данными.
func printCredentialRecord(out io.Writer, record clientApp.CredentialRecord) {
	_, _ = fmt.Fprintf(out, "* тип: credential\n")
	_, _ = fmt.Fprintf(out, "* название: %s\n", record.Title)
	_, _ = fmt.Fprintf(out, "* описание: %s\n", record.Description)
	_, _ = fmt.Fprintf(out, "* логин: %s\n", record.Login)
	_, _ = fmt.Fprintf(out, "* пароль: %s\n", record.Password)
}

// printTextRecord печатает расшифрованную текстовую запись.
func printTextRecord(out io.Writer, record clientApp.TextRecord) {
	_, _ = fmt.Fprintf(out, "* тип: text\n")
	_, _ = fmt.Fprintf(out, "* название: %s\n", record.Title)
	_, _ = fmt.Fprintf(out, "* описание: %s\n", record.Description)
	_, _ = fmt.Fprintf(out, "* текст: %s\n", record.Text)
}

// printCardRecord печатает расшифрованную запись банковской карты.
func printCardRecord(out io.Writer, record clientApp.CardRecord) {
	_, _ = fmt.Fprintf(out, "* тип: card\n")
	_, _ = fmt.Fprintf(out, "* название: %s\n", record.Title)
	_, _ = fmt.Fprintf(out, "* описание: %s\n", record.Description)
	_, _ = fmt.Fprintf(out, "* номер карты: %s\n", formatCardNumber(record.Number))
	_, _ = fmt.Fprintf(out, "* имя владельца: %s\n", record.HolderName)
	_, _ = fmt.Fprintf(out, "* срок действия: %s\n", record.ExpiresAt)
	_, _ = fmt.Fprintf(out, "* CVC: %s\n", record.CVC)
}

// printBinaryRecord печатает метаданные бинарной записи.
func printBinaryRecord(out io.Writer, record clientApp.BinaryRecord) {
	_, _ = fmt.Fprintf(out, "* тип: binary\n")
	_, _ = fmt.Fprintf(out, "* название: %s\n", record.Title)
	_, _ = fmt.Fprintf(out, "* описание: %s\n", record.Description)
	_, _ = fmt.Fprintf(out, "* исходное имя: %s\n", record.Filename)
	_, _ = fmt.Fprintf(out, "* MIME-тип: %s\n", record.ContentType)
	_, _ = fmt.Fprintf(out, "* размер: %d байт\n", record.Size)
	_, _ = fmt.Fprintf(out, "* статус загрузки: %s\n", record.UploadStatus)
}

// formatCardNumber форматирует номер карты группами по четыре цифры.
func formatCardNumber(number string) string {
	normalized, err := clientApp.NormalizeCardNumber(number)
	if err != nil || len(normalized) != 16 {
		return number
	}
	return normalized[:4] + " " + normalized[4:8] + " " + normalized[8:12] + " " + normalized[12:16]
}
