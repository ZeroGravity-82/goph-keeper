package model

import "errors"

var (
	// ErrRecordNotFound возвращается, когда приватная запись не найдена.
	ErrRecordNotFound = errors.New("record not found")
)
