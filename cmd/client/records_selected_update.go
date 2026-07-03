package main

import (
	"bufio"
	"context"
	"fmt"
	"io"

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
)

// updateSelectedCredential обновляет выбранную запись с учетными данными, используя версию из кеша.
func updateSelectedCredential(
	ctx context.Context,
	app *clientApp.App,
	state *recordsMenuState,
	item clientApp.RecordListItem,
	reader *bufio.Reader,
	out io.Writer,
) error {
	record, err := cachedCredential(ctx, app, state, item)
	if err != nil {
		return err
	}
	printCancelHint(out)
	title, err := promptWithDefaultCancelable(reader, out, "Новое название", record.Title)
	if err != nil {
		return err
	}
	description, err := promptWithDefaultCancelable(reader, out, "Новое описание", record.Description)
	if err != nil {
		return err
	}
	login, err := promptWithDefaultCancelable(
		reader,
		out,
		"Новый логин",
		record.Login,
	)
	if err != nil {
		return err
	}
	password, err := promptWithDefaultCancelable(
		reader,
		out,
		"Новый пароль",
		record.Password,
	)
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	updated, err := app.UpdateCredential(callCtx, clientApp.UpdateCredentialInput{
		RecordID:           record.RecordID,
		ExpectedVersion:    record.Version,
		Title:              title,
		Description:        description,
		CredentialLogin:    login,
		CredentialPassword: password,
	})
	if err != nil {
		return err
	}
	record.Title = title
	record.Description = description
	record.Login = login
	record.Password = password
	record.Version = updated.Version
	state.storeCredential(record)
	fmt.Fprintf(out, "обновлена приватная запись с учетными данными: %s\n", record.Title)
	return nil
}

// updateSelectedText обновляет выбранную текстовую запись, используя версию из кеша.
func updateSelectedText(
	ctx context.Context,
	app *clientApp.App,
	state *recordsMenuState,
	item clientApp.RecordListItem,
	reader *bufio.Reader,
	out io.Writer,
) error {
	record, err := cachedText(ctx, app, state, item)
	if err != nil {
		return err
	}
	printCancelHint(out)
	title, err := promptWithDefaultCancelable(reader, out, "Новое название", record.Title)
	if err != nil {
		return err
	}
	description, err := promptWithDefaultCancelable(reader, out, "Новое описание", record.Description)
	if err != nil {
		return err
	}
	text, err := promptWithDefaultCancelable(reader, out, "Новый текст", record.Text)
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	updated, err := app.UpdateText(callCtx, clientApp.UpdateTextInput{
		RecordID:        record.RecordID,
		ExpectedVersion: record.Version,
		Title:           title,
		Description:     description,
		Text:            text,
	})
	if err != nil {
		return err
	}
	record.Title = title
	record.Description = description
	record.Text = text
	record.Version = updated.Version
	state.storeText(record)
	fmt.Fprintf(out, "обновлена текстовая приватная запись: %s\n", record.Title)
	return nil
}

// updateSelectedCard обновляет выбранную запись банковской карты, используя версию из кеша.
func updateSelectedCard(
	ctx context.Context,
	app *clientApp.App,
	state *recordsMenuState,
	item clientApp.RecordListItem,
	reader *bufio.Reader,
	out io.Writer,
) error {
	record, err := cachedCard(ctx, app, state, item)
	if err != nil {
		return err
	}
	printCancelHint(out)
	title, err := promptWithDefaultCancelable(reader, out, "Новое название", record.Title)
	if err != nil {
		return err
	}
	description, err := promptWithDefaultCancelable(reader, out, "Новое описание", record.Description)
	if err != nil {
		return err
	}
	number, err := promptCardNumberWithDefault(reader, out, "Новый номер карты", record.Number)
	if err != nil {
		return err
	}
	holderName, err := promptCardHolderNameWithDefault(reader, out, "Новое имя владельца", record.HolderName)
	if err != nil {
		return err
	}
	expiresAt, err := promptCardExpirationWithDefault(reader, out, "Новый срок действия", record.ExpiresAt)
	if err != nil {
		return err
	}
	cvc, err := promptCardCVCWithDefault(reader, out, "Новый CVC", record.CVC)
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	updated, err := app.UpdateCard(callCtx, clientApp.UpdateCardInput{
		RecordID:        record.RecordID,
		ExpectedVersion: record.Version,
		Title:           title,
		Description:     description,
		Number:          number,
		HolderName:      holderName,
		ExpiresAt:       expiresAt,
		CVC:             cvc,
	})
	if err != nil {
		return err
	}
	record.Title = title
	record.Description = description
	record.Number = number
	record.HolderName = holderName
	record.ExpiresAt = expiresAt
	record.CVC = cvc
	record.Version = updated.Version
	state.storeCard(record)
	fmt.Fprintf(out, "обновлена приватная запись банковской карты: %s\n", record.Title)
	return nil
}

// updateSelectedBinaryMetadata обновляет метаданные выбранной бинарной записи, используя версию из кеша.
func updateSelectedBinaryMetadata(
	ctx context.Context,
	app *clientApp.App,
	state *recordsMenuState,
	item clientApp.RecordListItem,
	reader *bufio.Reader,
	out io.Writer,
) error {
	record, err := cachedBinary(ctx, app, state, item)
	if err != nil {
		return err
	}
	printCancelHint(out)
	title, err := promptWithDefaultCancelable(reader, out, "Новое название", record.Title)
	if err != nil {
		return err
	}
	description, err := promptWithDefaultCancelable(reader, out, "Новое описание", record.Description)
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	updated, err := app.UpdateBinaryMetadata(callCtx, clientApp.UpdateBinaryMetadataInput{
		RecordID:        record.RecordID,
		ExpectedVersion: record.Version,
		Title:           title,
		Description:     description,
	})
	if err != nil {
		return err
	}
	record.Title = title
	record.Description = description
	record.Version = updated.Version
	state.storeBinary(record)
	fmt.Fprintf(out, "обновлены метаданные бинарной приватной записи: %s\n", record.Title)
	return nil
}
