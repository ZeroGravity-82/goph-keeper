package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
)

// runRecordsMenu запускает меню действий с приватными записями для активной пользовательской сессии.
func runRecordsMenu(
	ctx context.Context,
	app *clientApp.App,
	reader *bufio.Reader,
	in io.Reader,
	out io.Writer,
	login string,
) error {
	state := newRecordsMenuState()
	if err := refreshRecordsMenu(ctx, app, state, out); err != nil {
		printError(out, err)
	}
	welcomeLogin := login
	for {
		clearScreen(out)
		if welcomeLogin != "" {
			printWelcome(out, welcomeLogin)
			fmt.Fprintln(out)
			welcomeLogin = ""
		}
		printRecordsMenu(out, state.items, state.readonly)
		fmt.Fprintln(out)
		choice, err := promptRequiredRetry(reader, out, "Выберите действие: ", "действие обязательно")
		if err != nil {
			return err
		}

		switch choice {
		case "1":
			if err := operateSelectedRecord(ctx, app, state, reader, out); err != nil {
				if errors.Is(err, errBackToRecordsList) {
					continue
				}
				handleConnectionError(state, out, err)
				printActionError(out, err)
			}
		case "2":
			if err := state.ensureWritable(); err != nil {
				printActionError(out, err)
				break
			}
			if err := createCredential(ctx, app, reader, out); err != nil {
				handleConnectionError(state, out, err)
				printActionError(out, err)
				break
			}
			if err := refreshRecordsMenu(ctx, app, state, out); err != nil {
				printError(out, err)
			}
		case "3":
			if err := state.ensureWritable(); err != nil {
				printActionError(out, err)
				break
			}
			if err := createText(ctx, app, reader, out); err != nil {
				handleConnectionError(state, out, err)
				printActionError(out, err)
				break
			}
			if err := refreshRecordsMenu(ctx, app, state, out); err != nil {
				printError(out, err)
			}
		case "4":
			if err := state.ensureWritable(); err != nil {
				printActionError(out, err)
				break
			}
			if err := createCard(ctx, app, reader, out); err != nil {
				handleConnectionError(state, out, err)
				printActionError(out, err)
				break
			}
			if err := refreshRecordsMenu(ctx, app, state, out); err != nil {
				printError(out, err)
			}
		case "5":
			if err := state.ensureWritable(); err != nil {
				printActionError(out, err)
				break
			}
			if err := createBinary(ctx, app, reader, out); err != nil {
				handleConnectionError(state, out, err)
				printActionError(out, err)
				refreshRecordsAfterBinaryFailure(ctx, app, state)
				break
			}
			if err := refreshRecordsMenu(ctx, app, state, out); err != nil {
				printError(out, err)
			}
		case "6":
			wasReadonly := state.readonly
			if err := refreshRecordsMenu(ctx, app, state, out); err != nil {
				printError(out, err)
				break
			}
			if wasReadonly {
				break
			}
			continue
		case "7":
			if err := state.ensureWritable(); err != nil {
				printActionError(out, err)
				break
			}
			if err := changeMasterKey(ctx, app, reader, in, out); err != nil {
				handleConnectionError(state, out, err)
				printActionError(out, err)
				break
			}
			state = newRecordsMenuState()
			if err := refreshRecordsMenu(ctx, app, state, out); err != nil {
				printError(out, err)
			}
		case "8":
			confirmed, err := confirm(reader, out, "Выйти из аккаунта?")
			if err != nil {
				return err
			}
			if !confirmed {
				continue
			}
			if err := logout(ctx, app, out); err != nil {
				printError(out, err)
			}
			return nil
		case "9":
			confirmed, err := confirm(reader, out, "Завершить приложение?")
			if err != nil {
				return err
			}
			if !confirmed {
				continue
			}
			if err := logoutAndExit(ctx, app, out); err != nil {
				return err
			}
			return errExitApplication
		default:
			fmt.Fprintln(out, "неизвестное действие")
		}
		if err := waitForEnter(reader, out); err != nil {
			return err
		}
		pauseBeforeClearScreen(out)
	}
}

// printRecordsMenu печатает меню действий, доступных после успешного входа в аккаунт.
func printRecordsMenu(out io.Writer, items []clientApp.RecordListItem, readonly bool) {
	printRecordsTable(out, items)
	if readonly {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Режим чтения: создание, изменение и удаление записей временно недоступны.")
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Действия:")
	fmt.Fprintln(out, "1. Выполнить действие над записью из списка")
	fmt.Fprintln(out, "2. Создать запись с учетными данными")
	fmt.Fprintln(out, "3. Создать текстовую запись")
	fmt.Fprintln(out, "4. Создать запись для банковской карты")
	fmt.Fprintln(out, "5. Создать бинарную запись с файлом")
	fmt.Fprintln(out, "6. Обновить список записей")
	fmt.Fprintln(out, "7. Сменить мастер-ключ")
	fmt.Fprintln(out, "8. Выйти из аккаунта")
	fmt.Fprintln(out, "9. Завершить приложение")
}

// refreshRecordsMenu обновляет список записей и переключает режим чтения по результату запроса.
func refreshRecordsMenu(ctx context.Context, app *clientApp.App, state *recordsMenuState, out io.Writer) error {
	if err := state.refresh(ctx, app); err != nil {
		handleConnectionError(state, out, err)
		return err
	}
	state.exitReadonly(out)
	return nil
}

// refreshRecordsAfterBinaryFailure тихо обновляет список после неуспешной файловой загрузки.
func refreshRecordsAfterBinaryFailure(ctx context.Context, app *clientApp.App, state *recordsMenuState) {
	if app == nil || state == nil {
		return
	}
	_ = state.refresh(ctx, app)
}

// handleConnectionError переводит меню в режим чтения при ошибке связи с сервером.
func handleConnectionError(state *recordsMenuState, out io.Writer, err error) {
	if clientApp.IsConnectionError(err) {
		if state.readonly {
			fmt.Fprintln(out, "режим чтения: связь с сервером все еще недоступна")
			return
		}
		state.enterReadonly(out)
	}
}

// operateSelectedRecord открывает выбранную пользователем запись и выполняет действие над ней.
func operateSelectedRecord(
	ctx context.Context,
	app *clientApp.App,
	state *recordsMenuState,
	reader *bufio.Reader,
	out io.Writer,
) error {
	if len(state.items) == 0 {
		return errors.New("список приватных записей пуст")
	}
	row, err := promptRecordRow(reader, out, len(state.items))
	if err != nil {
		return err
	}
	item := state.items[row-1]
	if err = openSelectedRecord(ctx, app, state, item, out); err != nil {
		return err
	}
	fmt.Fprintln(out)
	printRecordActionMenu(out, item.Type)
	fmt.Fprintln(out)
	choice, err := promptRequiredRetry(reader, out, "Выберите действие: ", "действие обязательно")
	if err != nil {
		return err
	}
	switch choice {
	case "0":
		return errBackToRecordsList
	case "1":
		if err := state.ensureWritable(); err != nil {
			return err
		}
		return updateSelectedRecord(ctx, app, state, item, reader, out)
	case "2":
		if err := state.ensureWritable(); err != nil {
			return err
		}
		return deleteSelectedRecord(ctx, app, state, item, reader, out)
	case "3":
		if item.Type == "binary" {
			if err := state.ensureWritable(); err != nil {
				return err
			}
			return downloadSelectedBinaryFile(ctx, app, item, reader, out)
		}
	case "4":
		if item.Type == "binary" {
			if err := state.ensureWritable(); err != nil {
				return err
			}
			return replaceSelectedBinaryFile(ctx, app, state, item, reader, out)
		}
	}
	return errors.New("неизвестное действие")
}

// promptRecordRow запрашивает номер строки записи из текущего списка.
func promptRecordRow(reader *bufio.Reader, out io.Writer, maxRow int) (int, error) {
	printCancelHint(out)
	value, err := promptRequiredRetryCancelable(reader, out, "Выберите запись: ", "номер записи обязателен")
	if err != nil {
		return 0, err
	}
	row, err := strconv.Atoi(value)
	if err != nil || row < 1 || row > maxRow {
		if maxRow == 1 {
			return 0, errors.New("номер записи должен быть 1")
		}
		return 0, fmt.Errorf("номер записи должен быть от 1 до %d", maxRow)
	}
	return row, nil
}

// printRecordActionMenu печатает действия, доступные для выбранной записи.
func printRecordActionMenu(out io.Writer, recordType string) {
	fmt.Fprintln(out, "Действия с записью:")
	fmt.Fprintln(out, "0. Вернуться к списку")
	fmt.Fprintln(out, "1. Изменить")
	fmt.Fprintln(out, "2. Удалить")
	if recordType == "binary" {
		fmt.Fprintln(out, "3. Скачать файл")
		fmt.Fprintln(out, "4. Заменить файл")
	}
}

// openSelectedRecord загружает полные данные выбранной записи, обновляет кеш и печатает их.
func openSelectedRecord(
	ctx context.Context,
	app *clientApp.App,
	state *recordsMenuState,
	item clientApp.RecordListItem,
	out io.Writer,
) error {
	if state.readonly {
		return openCachedSelectedRecord(state, item, out)
	}
	switch item.Type {
	case "credential":
		record, err := loadCredential(ctx, app, item.RecordID)
		if err != nil {
			return openCachedAfterConnectionError(state, item, out, err)
		}
		state.storeCredential(record)
		printCredentialRecord(out, record)
	case "text":
		record, err := loadText(ctx, app, item.RecordID)
		if err != nil {
			return openCachedAfterConnectionError(state, item, out, err)
		}
		state.storeText(record)
		printTextRecord(out, record)
	case "card":
		record, err := loadCard(ctx, app, item.RecordID)
		if err != nil {
			return openCachedAfterConnectionError(state, item, out, err)
		}
		state.storeCard(record)
		printCardRecord(out, record)
	case "binary":
		record, err := loadBinary(ctx, app, item.RecordID)
		if err != nil {
			return openCachedAfterConnectionError(state, item, out, err)
		}
		state.storeBinary(record)
		printBinaryRecord(out, record)
	default:
		return fmt.Errorf("неподдерживаемый тип приватной записи: %s", item.Type)
	}
	return nil
}

// openCachedAfterConnectionError включает режим чтения при потере связи и пытается открыть запись из кеша.
func openCachedAfterConnectionError(
	state *recordsMenuState,
	item clientApp.RecordListItem,
	out io.Writer,
	err error,
) error {
	if !clientApp.IsConnectionError(err) {
		return err
	}
	state.enterReadonly(out)
	return openCachedSelectedRecord(state, item, out)
}

// openCachedSelectedRecord печатает полные данные записи, если они уже были загружены за текущий запуск приложения.
func openCachedSelectedRecord(state *recordsMenuState, item clientApp.RecordListItem, out io.Writer) error {
	entry, ok := state.cache[item.RecordID]
	if !ok {
		return errors.New("запись не загружена за текущий запуск приложения")
	}
	switch item.Type {
	case "credential":
		if entry.credential == nil {
			return errors.New("запись не загружена за текущий запуск приложения")
		}
		printCredentialRecord(out, *entry.credential)
	case "text":
		if entry.text == nil {
			return errors.New("запись не загружена за текущий запуск приложения")
		}
		printTextRecord(out, *entry.text)
	case "card":
		if entry.card == nil {
			return errors.New("запись не загружена за текущий запуск приложения")
		}
		printCardRecord(out, *entry.card)
	case "binary":
		if entry.binary == nil {
			return errors.New("запись не загружена за текущий запуск приложения")
		}
		printBinaryRecord(out, *entry.binary)
	default:
		return fmt.Errorf("неподдерживаемый тип приватной записи: %s", item.Type)
	}
	return nil
}

// updateSelectedRecord выбирает сценарий обновления по типу выбранной записи.
func updateSelectedRecord(
	ctx context.Context,
	app *clientApp.App,
	state *recordsMenuState,
	item clientApp.RecordListItem,
	reader *bufio.Reader,
	out io.Writer,
) error {
	switch item.Type {
	case "credential":
		return updateSelectedCredential(ctx, app, state, item, reader, out)
	case "text":
		return updateSelectedText(ctx, app, state, item, reader, out)
	case "card":
		return updateSelectedCard(ctx, app, state, item, reader, out)
	case "binary":
		return updateSelectedBinaryMetadata(ctx, app, state, item, reader, out)
	default:
		return fmt.Errorf("неподдерживаемый тип приватной записи: %s", item.Type)
	}
}

// deleteSelectedRecord удаляет выбранную запись после подтверждения пользователем.
func deleteSelectedRecord(
	ctx context.Context,
	app *clientApp.App,
	state *recordsMenuState,
	item clientApp.RecordListItem,
	reader *bufio.Reader,
	out io.Writer,
) error {
	confirmed, err := confirm(reader, out, fmt.Sprintf("Удалить запись %q?", item.Title))
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Fprintln(out, "удаление отменено")
		return nil
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	deleted, err := app.DeleteRecord(callCtx, item.RecordID)
	if err != nil {
		return err
	}
	delete(state.cache, deleted.RecordID)
	for i := range state.items {
		if state.items[i].RecordID == deleted.RecordID {
			state.items = append(state.items[:i], state.items[i+1:]...)
			break
		}
	}
	fmt.Fprintf(out, "приватная запись удалена: %s\n", item.Title)
	return nil
}
