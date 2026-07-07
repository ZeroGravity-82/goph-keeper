package usecase

import "errors"

// ErrBinaryRecordNotSupported возвращается, когда бинарную приватную запись пытаются создать через неподходящий
// сценарий.
var ErrBinaryRecordNotSupported = errors.New("binary record is not supported by this operation")

// ErrBinaryEncryptedSizeMismatch возвращается, когда фактический размер файла не совпадает с ожидаемым.
var ErrBinaryEncryptedSizeMismatch = errors.New("binary encrypted size does not match uploaded data")

// ErrRecordIsNotBinary возвращается, когда файловую операцию пытаются выполнить для обычной приватной записи.
var ErrRecordIsNotBinary = errors.New("record is not binary")

// ErrRecordFileIsNotUploaded возвращается, когда файл приватной записи еще не был успешно загружен.
var ErrRecordFileIsNotUploaded = errors.New("record file is not uploaded")

// ErrRecordNotFound возвращается, если приватная запись не найдена.
var ErrRecordNotFound = errors.New("record not found")

// ErrRecordVersionConflict возвращается, если приватная запись была параллельно изменена с другого клиента.
var ErrRecordVersionConflict = errors.New("record version conflict")
