package usecase

import "errors"

// ErrAuthenticationFailed возвращается при неуспешной аутентификации.
var ErrAuthenticationFailed = errors.New("authentication failed")

// ErrLoginAlreadyTaken возвращается при попытке зарегистрировать уже занятый логин.
var ErrLoginAlreadyTaken = errors.New("login already taken")

// ErrRefreshTokenNotFound возвращается, когда активный refresh-токен не найден.
var ErrRefreshTokenNotFound = errors.New("refresh token not found")

// ErrInvalidMasterKeySalt возвращается, когда соль мастер-ключа имеет некорректный формат.
var ErrInvalidMasterKeySalt = errors.New("invalid master key salt")

// ErrUserNotFound возвращается, когда пользователь не найден.
var ErrUserNotFound = errors.New("user not found")
