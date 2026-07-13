// Package migrations встраивает SQL-миграции базы данных в бинарный файл сервера.
package migrations

import "embed"

// FS содержит файлы миграций, встроенные в бинарный файл сервиса.
//
//go:embed *.sql
var FS embed.FS
