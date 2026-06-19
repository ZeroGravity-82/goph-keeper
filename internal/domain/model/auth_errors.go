package model

import "errors"

var (
	// ErrRefreshTokenNotFound возвращается, когда активный refresh-токен не найден.
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
)
