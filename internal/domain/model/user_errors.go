package model

import "errors"

var (
	// ErrAuthenticationFailed возвращается при неуспешной аутентификации.
	ErrAuthenticationFailed = errors.New("authentication failed")
	// ErrLoginAlreadyTaken возвращается при попытке зарегистрировать уже занятый логин.
	ErrLoginAlreadyTaken = errors.New("login already taken")
	// ErrUserNotFound возвращается, когда пользователь не найден.
	ErrUserNotFound = errors.New("user not found")
)
