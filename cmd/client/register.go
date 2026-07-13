package main

import (
	"bufio"
	"context"
	"io"

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
)

// register регистрирует пользователя, проверяет мастер-ключ и сразу открывает пользовательскую сессию.
func register(
	ctx context.Context,
	app *clientApp.App,
	reader *bufio.Reader,
	in io.Reader,
	out io.Writer,
) (string, bool, error) {
	login, err := promptRequired(reader, out, "Логин: ")
	if err != nil {
		return "", false, err
	}
	password, err := promptSecretConfirmedRequired(
		reader,
		in,
		out,
		"Пароль: ",
		"Повторите пароль: ",
		"пароль обязателен",
	)
	if err != nil {
		return "", false, err
	}

	masterKey, err := promptMasterKeyConfirmed(reader, in, out)
	if err != nil {
		return "", false, err
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	session, err := app.Register(callCtx, login, password, masterKey)
	cancel()
	if err != nil {
		return "", false, err
	}
	if err = app.StartSession(session, masterKey); err != nil {
		return "", false, err
	}

	return login, true, nil
}
