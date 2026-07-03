package main

import (
	"bufio"
	"context"
	"fmt"
	"io"

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
)

// createCredential запрашивает поля учетных данных и создает зашифрованную приватную запись.
func createCredential(ctx context.Context, app *clientApp.App, reader *bufio.Reader, out io.Writer) error {
	printCancelHint(out)
	title, err := promptRequiredCancelable(reader, out, "Название: ")
	if err != nil {
		return err
	}
	description, err := promptCancelable(reader, out, "Описание: ")
	if err != nil {
		return err
	}
	credentialLogin, err := promptRequiredCancelable(reader, out, "Логин: ")
	if err != nil {
		return err
	}
	credentialPassword, err := promptRequiredCancelable(reader, out, "Пароль: ")
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	_, err = app.CreateCredential(callCtx, clientApp.CreateCredentialInput{
		Title:              title,
		Description:        description,
		CredentialLogin:    credentialLogin,
		CredentialPassword: credentialPassword,
	})
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "создана приватная запись с учетными данными")
	return nil
}
