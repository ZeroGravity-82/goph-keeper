package main

import (
	"bufio"
	"context"
	"fmt"
	"io"

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
)

// createCard запрашивает поля банковской карты и создает зашифрованную приватную запись.
func createCard(ctx context.Context, app *clientApp.App, reader *bufio.Reader, out io.Writer) error {
	printCancelHint(out)
	title, err := promptRequiredCancelable(reader, out, "Название: ")
	if err != nil {
		return err
	}
	description, err := promptCancelable(reader, out, "Описание: ")
	if err != nil {
		return err
	}
	number, err := promptCardNumber(reader, out, "Номер карты: ")
	if err != nil {
		return err
	}
	holderName, err := promptCardHolderName(reader, out, "Имя владельца: ")
	if err != nil {
		return err
	}
	expiresAt, err := promptCardExpiration(reader, out, "Срок действия: ")
	if err != nil {
		return err
	}
	cvc, err := promptCardCVC(reader, out, "CVC: ")
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	_, err = app.CreateCard(callCtx, clientApp.CreateCardInput{
		Title:       title,
		Description: description,
		Number:      number,
		HolderName:  holderName,
		ExpiresAt:   expiresAt,
		CVC:         cvc,
	})
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "создана приватная запись банковской карты")
	return nil
}
