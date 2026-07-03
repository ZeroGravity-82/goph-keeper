package main

import (
	"bufio"
	"context"
	"fmt"
	"io"

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
)

// createText запрашивает поля текстовой записи и создает зашифрованную приватную запись.
func createText(ctx context.Context, app *clientApp.App, reader *bufio.Reader, out io.Writer) error {
	printCancelHint(out)
	title, err := promptRequiredCancelable(reader, out, "Название: ")
	if err != nil {
		return err
	}
	description, err := promptCancelable(reader, out, "Описание: ")
	if err != nil {
		return err
	}
	text, err := promptRequiredCancelable(reader, out, "Текст: ")
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	_, err = app.CreateText(callCtx, clientApp.CreateTextInput{
		Title:       title,
		Description: description,
		Text:        text,
	})
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "создана текстовая приватная запись")
	return nil
}
