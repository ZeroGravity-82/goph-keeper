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

// ErrMultipartUploadIncomplete возвращается, когда multipart-загрузку пытаются завершить без всех частей файла.
var ErrMultipartUploadIncomplete = errors.New("multipart upload is incomplete")

// ErrMultipartUploadNotActive возвращается, когда сессия multipart-загрузки уже завершена, отменена или помечена
// неуспешной.
var ErrMultipartUploadNotActive = errors.New("multipart upload is not active")

// ErrMultipartUploadNotFound возвращается, если сессия multipart-загрузки не найдена.
var ErrMultipartUploadNotFound = errors.New("multipart upload not found")

// ErrMultipartUploadPartInvalid возвращается, когда часть multipart-загрузки не соответствует ожидаемым параметрам.
var ErrMultipartUploadPartInvalid = errors.New("multipart upload part is invalid")

// ErrRecordNotFound возвращается, если приватная запись не найдена.
var ErrRecordNotFound = errors.New("record not found")

// ErrRecordVersionConflict возвращается, если приватная запись была параллельно изменена с другого клиента.
var ErrRecordVersionConflict = errors.New("record version conflict")
