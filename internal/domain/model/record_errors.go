package model

import "errors"

var (
	// ErrRecordNotFound возвращается, если запись пользователя не найдена.
	ErrRecordNotFound = errors.New("record not found")
)
