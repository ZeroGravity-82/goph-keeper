package migrations

import "embed"

// FS содержит файлы миграций, встроенные в бинарный файл сервиса.
//
//go:embed *.sql
var FS embed.FS
