package main

import (
	"bufio"
	"context"
	"fmt"
	"io"

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
)

// changeMasterKey запрашивает текущий и новый мастер-ключи и запускает смену мастер-ключа на клиенте и сервере.
func changeMasterKey(
	ctx context.Context,
	app *clientApp.App,
	reader *bufio.Reader,
	in io.Reader,
	out io.Writer,
) error {
	printCancelHint(out)
	currentMasterKey, err := promptSecret(reader, in, out, "Текущий мастер-ключ: ")
	if err != nil {
		return err
	}
	if isCancelInput(currentMasterKey) {
		return errActionCanceled
	}
	if currentMasterKey == "" {
		return requiredInputError{message: "текущий мастер-ключ обязателен"}
	}
	newMasterKey, err := promptSecret(reader, in, out, "Новый мастер-ключ: ")
	if err != nil {
		return err
	}
	if isCancelInput(newMasterKey) {
		return errActionCanceled
	}
	if newMasterKey == "" {
		return requiredInputError{message: "новый мастер-ключ обязателен"}
	}
	confirmedMasterKey, err := promptSecret(reader, in, out, "Повторите новый мастер-ключ: ")
	if err != nil {
		return err
	}
	if isCancelInput(confirmedMasterKey) {
		return errActionCanceled
	}
	if newMasterKey != confirmedMasterKey {
		return fmt.Errorf("значения не совпадают")
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	if err = app.ChangeMasterKey(callCtx, currentMasterKey, newMasterKey); err != nil {
		return err
	}
	fmt.Fprintln(out, "мастер-ключ изменен")
	return nil
}
