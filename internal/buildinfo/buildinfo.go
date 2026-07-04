// Пакет buildinfo форматирует метаданные сборки приложения для вывода в CLI.
package buildinfo

import "fmt"

const unknownValue = "N/A"

// Info содержит версию и дату сборки бинарного файла.
type Info struct {
	Version string
	Date    string
}

// New создает Info и подставляет N/A вместо пустых значений.
func New(version string, date string) Info {
	if version == "" {
		version = unknownValue
	}
	if date == "" {
		date = unknownValue
	}
	return Info{Version: version, Date: date}
}

// Title возвращает строку заголовка приложения с версией и датой сборки.
func (i Info) Title(appName string) string {
	return fmt.Sprintf("%s (версия: %s, дата сборки: %s)", appName, i.Version, i.Date)
}
