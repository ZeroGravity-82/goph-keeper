package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
)

// runStartMenu запускает стартовое меню CLI-клиента до регистрации, входа в аккаунт или выхода из приложения.
func runStartMenu(ctx context.Context, app *clientApp.App, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	for {
		printStartMenu(out)
		choice, err := promptRequiredRetry(reader, out, "Выберите действие: ", "действие обязательно")
		if err != nil {
			return err
		}

		switch choice {
		case "1":
			loggedIn, err := register(ctx, app, reader, in, out)
			if err != nil {
				printError(out, err)
				continue
			}
			if loggedIn {
				if err = runRecordsMenu(ctx, app, reader, out); err != nil {
					return err
				}
			}
		case "2":
			loggedIn, err := login(ctx, app, reader, in, out)
			if err != nil {
				printError(out, err)
				continue
			}
			if loggedIn {
				if err = runRecordsMenu(ctx, app, reader, out); err != nil {
					return err
				}
			}
		case "3":
			return nil
		default:
			fmt.Fprintln(out, "неизвестное действие")
		}
	}
}

// printStartMenu печатает меню действий, доступных без активной пользовательской сессии.
func printStartMenu(out io.Writer) {
	_, _ = fmt.Fprintln(out, `
1. Зарегистрироваться
2. Войти в аккаунт
3. Выйти из приложения`)
}

// login выполняет вход пользователя, проверяет мастер-ключ и открывает клиентскую сессию.
func login(ctx context.Context, app *clientApp.App, reader *bufio.Reader, in io.Reader, out io.Writer) (bool, error) {
	login, err := promptRequired(reader, out, "Логин: ")
	if err != nil {
		return false, err
	}
	password, err := promptSecret(reader, in, out, "Пароль: ")
	if err != nil {
		return false, err
	}
	if password == "" {
		return false, errors.New("пароль обязателен")
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	session, err := app.Login(callCtx, login, password)
	cancel()
	if err != nil {
		return false, err
	}

	session, masterKey, err := prepareMasterKey(session, reader, in, out)
	if err != nil {
		return false, err
	}
	if err = app.StartSession(session, masterKey); err != nil {
		return false, err
	}

	fmt.Fprintf(out, "выполнен вход: %q\n", login)
	printWelcome(out, login)
	return true, nil
}

// printWelcome печатает приветствие после успешной аутентификации.
func printWelcome(out io.Writer, login string) {
	fmt.Fprintf(out, "Добро пожаловать в Goph Keeper, %s.\n", login)
}

// promptMasterKey запрашивает мастер-ключ без подтверждения.
func promptMasterKey(reader *bufio.Reader, in io.Reader, out io.Writer) (string, error) {
	masterKey, err := promptSecret(reader, in, out, "Мастер-ключ: ")
	if err != nil {
		return "", err
	}
	if masterKey == "" {
		return "", errors.New("мастер-ключ обязателен")
	}
	return masterKey, nil
}

// promptMasterKeyConfirmed запрашивает мастер-ключ с повторным вводом для подтверждения.
func promptMasterKeyConfirmed(reader *bufio.Reader, in io.Reader, out io.Writer) (string, error) {
	masterKey, err := promptSecretConfirmed(
		reader,
		in,
		out,
		"Мастер-ключ: ",
		"Повторите мастер-ключ: ",
	)
	if err != nil {
		return "", err
	}
	if masterKey == "" {
		return "", errors.New("мастер-ключ обязателен")
	}
	return masterKey, nil
}

// prepareMasterKey получает мастер-ключ и проверяет, что сервер вернул данные для его проверки.
func prepareMasterKey(
	session clientApp.AuthSession,
	reader *bufio.Reader,
	in io.Reader,
	out io.Writer,
) (clientApp.AuthSession, string, error) {
	if len(session.MasterKeyVerifier) > 0 {
		masterKey, err := promptMasterKey(reader, in, out)
		return session, masterKey, err
	}
	return clientApp.AuthSession{}, "", errors.New("проверочные данные мастер-ключа отсутствуют")
}

// runRecordsMenu запускает меню операций с приватными записями для активной пользовательской сессии.
func runRecordsMenu(ctx context.Context, app *clientApp.App, reader *bufio.Reader, out io.Writer) error {
	state := newRecordsMenuState()
	if err := state.refresh(ctx, app); err != nil {
		printError(out, err)
	}
	for {
		clearScreen(out)
		printRecordsMenu(out, state.items)
		fmt.Fprintln(out)
		choice, err := promptRequiredRetry(reader, out, "Выберите действие: ", "действие обязательно")
		if err != nil {
			return err
		}

		switch choice {
		case "1":
			if err := createCredential(ctx, app, reader, out); err != nil {
				printError(out, err)
				break
			}
			if err := state.refresh(ctx, app); err != nil {
				printError(out, err)
			}
		case "2":
			if err := createText(ctx, app, reader, out); err != nil {
				printError(out, err)
				break
			}
			if err := state.refresh(ctx, app); err != nil {
				printError(out, err)
			}
		case "3":
			if err := createCard(ctx, app, reader, out); err != nil {
				printError(out, err)
				break
			}
			if err := state.refresh(ctx, app); err != nil {
				printError(out, err)
			}
		case "4":
			if err := createBinary(ctx, app, reader, out); err != nil {
				printError(out, err)
				break
			}
			if err := state.refresh(ctx, app); err != nil {
				printError(out, err)
			}
		case "5":
			if err := state.refresh(ctx, app); err != nil {
				printError(out, err)
			}
		case "6":
			if err := operateSelectedRecord(ctx, app, state, reader, out); err != nil {
				printError(out, err)
			}
		case "7":
			if err := logout(ctx, app, out); err != nil {
				printError(out, err)
			}
			return nil
		case "8":
			return logoutAndExit(ctx, app, out)
		default:
			fmt.Fprintln(out, "неизвестное действие")
		}
		if err := waitForEnter(reader, out); err != nil {
			return err
		}
	}
}

// printRecordsMenu печатает меню действий, доступных после успешного входа в аккаунт.
func printRecordsMenu(out io.Writer, items []clientApp.RecordListItem) {
	printRecordsTable(out, items)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Действия:")
	fmt.Fprintln(out, "1. Создать логин/пароль")
	fmt.Fprintln(out, "2. Создать текстовую запись")
	fmt.Fprintln(out, "3. Создать банковскую карту")
	fmt.Fprintln(out, "4. Создать бинарную запись с файлом")
	fmt.Fprintln(out, "5. Обновить список записей")
	fmt.Fprintln(out, "6. Выбрать запись из списка")
	fmt.Fprintln(out, "7. Выйти из аккаунта")
	fmt.Fprintln(out, "8. Завершить приложение")
}

type recordsMenuState struct {
	items []clientApp.RecordListItem
	cache map[string]cachedRecord
}

type cachedRecord struct {
	item       clientApp.RecordListItem
	credential *clientApp.CredentialRecord
	text       *clientApp.TextRecord
	card       *clientApp.CardRecord
	binary     *clientApp.BinaryRecord
}

func newRecordsMenuState() *recordsMenuState {
	return &recordsMenuState{cache: make(map[string]cachedRecord)}
}

func (s *recordsMenuState) refresh(ctx context.Context, app *clientApp.App) error {
	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	items, err := app.ListRecords(callCtx)
	if err != nil {
		return err
	}
	s.items = items
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		seen[item.RecordID] = struct{}{}
		entry := s.cache[item.RecordID]
		entry.item = item
		s.cache[item.RecordID] = entry
	}
	for recordID := range s.cache {
		if _, ok := seen[recordID]; !ok {
			delete(s.cache, recordID)
		}
	}
	return nil
}

func (s *recordsMenuState) storeCredential(record clientApp.CredentialRecord) {
	item := clientApp.RecordListItem{
		RecordID:    record.RecordID,
		Type:        "credential",
		Title:       record.Title,
		Description: record.Description,
	}
	s.storeItem(item)
	entry := s.cache[record.RecordID]
	entry.credential = &record
	s.cache[record.RecordID] = entry
}

func (s *recordsMenuState) storeText(record clientApp.TextRecord) {
	item := clientApp.RecordListItem{
		RecordID:    record.RecordID,
		Type:        "text",
		Title:       record.Title,
		Description: record.Description,
	}
	s.storeItem(item)
	entry := s.cache[record.RecordID]
	entry.text = &record
	s.cache[record.RecordID] = entry
}

func (s *recordsMenuState) storeCard(record clientApp.CardRecord) {
	item := clientApp.RecordListItem{
		RecordID:    record.RecordID,
		Type:        "card",
		Title:       record.Title,
		Description: record.Description,
	}
	s.storeItem(item)
	entry := s.cache[record.RecordID]
	entry.card = &record
	s.cache[record.RecordID] = entry
}

func (s *recordsMenuState) storeBinary(record clientApp.BinaryRecord) {
	item := clientApp.RecordListItem{
		RecordID:    record.RecordID,
		Type:        "binary",
		Title:       record.Title,
		Description: record.Description,
	}
	s.storeItem(item)
	entry := s.cache[record.RecordID]
	entry.binary = &record
	s.cache[record.RecordID] = entry
}

func (s *recordsMenuState) storeItem(item clientApp.RecordListItem) {
	for i := range s.items {
		if s.items[i].RecordID == item.RecordID {
			s.items[i].Type = item.Type
			s.items[i].Title = item.Title
			s.items[i].Description = item.Description
			return
		}
	}
	s.items = append(s.items, item)
}

func printRecordsTable(out io.Writer, items []clientApp.RecordListItem) {
	fmt.Fprintln(out, "Записи:")
	if len(items) == 0 {
		fmt.Fprintln(out, "Записей еще нет.")
		return
	}
	fmt.Fprintln(out, "# | Тип | Название | Описание")
	for i, item := range items {
		fmt.Fprintf(out, "%d | %s | %s | %s\n", i+1, item.Type, item.Title, item.Description)
	}
}

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
	printRecordOperationMenu(out, item.Type)
	fmt.Fprintln(out)
	choice, err := promptRequiredRetry(reader, out, "Выберите действие: ", "действие обязательно")
	if err != nil {
		return err
	}
	switch choice {
	case "0":
		return nil
	case "1":
		return updateSelectedRecord(ctx, app, state, item, reader, out)
	case "2":
		return deleteSelectedRecord(ctx, app, state, item, reader, out)
	case "3":
		if item.Type != "binary" {
			return errors.New("скачивание файла доступно только для бинарной записи")
		}
		return downloadSelectedBinaryFile(ctx, app, item, reader, out)
	case "4":
		if item.Type != "binary" {
			return errors.New("замена файла доступна только для бинарной записи")
		}
		return replaceSelectedBinaryFile(ctx, app, state, item, reader, out)
	default:
		return errors.New("неизвестное действие")
	}
}

func promptRecordRow(reader *bufio.Reader, out io.Writer, maxRow int) (int, error) {
	value, err := promptRequiredRetry(reader, out, "Выберите запись: ", "номер записи обязателен")
	if err != nil {
		return 0, err
	}
	row, err := strconv.Atoi(value)
	if err != nil || row < 1 || row > maxRow {
		return 0, fmt.Errorf("номер записи должен быть от 1 до %d", maxRow)
	}
	return row, nil
}

func printRecordOperationMenu(out io.Writer, recordType string) {
	fmt.Fprintln(out, "Действия с записью:")
	fmt.Fprintln(out, "0. Вернуться к списку")
	fmt.Fprintln(out, "1. Изменить")
	fmt.Fprintln(out, "2. Удалить")
	if recordType == "binary" {
		fmt.Fprintln(out, "3. Скачать файл")
		fmt.Fprintln(out, "4. Заменить файл")
	}
}

func openSelectedRecord(
	ctx context.Context,
	app *clientApp.App,
	state *recordsMenuState,
	item clientApp.RecordListItem,
	out io.Writer,
) error {
	switch item.Type {
	case "credential":
		record, err := loadCredential(ctx, app, item.RecordID)
		if err != nil {
			return err
		}
		state.storeCredential(record)
		printCredentialRecord(out, record)
	case "text":
		record, err := loadText(ctx, app, item.RecordID)
		if err != nil {
			return err
		}
		state.storeText(record)
		printTextRecord(out, record)
	case "card":
		record, err := loadCard(ctx, app, item.RecordID)
		if err != nil {
			return err
		}
		state.storeCard(record)
		printCardRecord(out, record)
	case "binary":
		record, err := loadBinary(ctx, app, item.RecordID)
		if err != nil {
			return err
		}
		state.storeBinary(record)
		printBinaryRecord(out, record)
	default:
		return fmt.Errorf("неподдерживаемый тип приватной записи: %s", item.Type)
	}
	return nil
}

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
	title, err := promptWithDefault(reader, out, "Новое название", record.Title)
	if err != nil {
		return err
	}
	description, err := promptWithDefault(reader, out, "Новое описание", record.Description)
	if err != nil {
		return err
	}
	login, err := promptWithDefault(reader, out, "Новый логин сохраняемой учетной записи", record.Login)
	if err != nil {
		return err
	}
	password, err := promptWithDefault(reader, out, "Новый пароль сохраняемой учетной записи", record.Password)
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
	title, err := promptWithDefault(reader, out, "Новое название", record.Title)
	if err != nil {
		return err
	}
	description, err := promptWithDefault(reader, out, "Новое описание", record.Description)
	if err != nil {
		return err
	}
	text, err := promptWithDefault(reader, out, "Новый текст", record.Text)
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
	title, err := promptWithDefault(reader, out, "Новое название", record.Title)
	if err != nil {
		return err
	}
	description, err := promptWithDefault(reader, out, "Новое описание", record.Description)
	if err != nil {
		return err
	}
	number, err := promptWithDefault(reader, out, "Новый номер карты", record.Number)
	if err != nil {
		return err
	}
	holderName, err := promptWithDefault(reader, out, "Новое имя владельца", record.HolderName)
	if err != nil {
		return err
	}
	expiresAt, err := promptWithDefault(reader, out, "Новый срок действия", record.ExpiresAt)
	if err != nil {
		return err
	}
	cvc, err := promptWithDefault(reader, out, "Новый CVC", record.CVC)
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
	title, err := promptWithDefault(reader, out, "Новое название", record.Title)
	if err != nil {
		return err
	}
	description, err := promptWithDefault(reader, out, "Новое описание", record.Description)
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

func deleteSelectedRecord(
	ctx context.Context,
	app *clientApp.App,
	state *recordsMenuState,
	item clientApp.RecordListItem,
	reader *bufio.Reader,
	out io.Writer,
) error {
	confirmed, err := prompt(reader, out, fmt.Sprintf("Удалить запись %q? [y/N]: ", item.Title))
	if err != nil {
		return err
	}
	if strings.ToLower(confirmed) != "y" {
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

func downloadSelectedBinaryFile(
	ctx context.Context,
	app *clientApp.App,
	item clientApp.RecordListItem,
	reader *bufio.Reader,
	out io.Writer,
) error {
	outputDir, err := promptRequiredRetry(
		reader,
		out,
		"Директория для сохранения файла: ",
		"директория для сохранения файла обязательна",
	)
	if err != nil {
		return err
	}
	if err = ensureDirectory(outputDir); err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	file, err := app.DownloadBinaryFile(callCtx, item.RecordID)
	if err != nil {
		return err
	}
	outputPath := filepath.Join(outputDir, file.Filename)
	if err = os.WriteFile(outputPath, file.Data, 0o600); err != nil {
		return fmt.Errorf("не удалось сохранить файл: %w", err)
	}
	fmt.Fprintf(
		out,
		"файл сохранен: %s\nисходное имя: %s\nMIME-тип: %s\nразмер: %d байт\n",
		outputPath,
		file.Filename,
		file.ContentType,
		file.DeclaredSize,
	)
	return nil
}

func ensureDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("не удалось проверить директорию для сохранения файла: %w", err)
	}
	if !info.IsDir() {
		return errors.New("путь для сохранения файла должен быть директорией")
	}
	return nil
}

func replaceSelectedBinaryFile(
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
	title, err := promptWithDefault(reader, out, "Новое название", record.Title)
	if err != nil {
		return err
	}
	description, err := promptWithDefault(reader, out, "Новое описание", record.Description)
	if err != nil {
		return err
	}
	path, err := promptRequiredRetry(reader, out, "Путь к новому файлу: ", "путь к новому файлу обязателен")
	if err != nil {
		return err
	}
	file, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("не удалось прочитать файл: %w", err)
	}
	filename := filepath.Base(path)
	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	updated, err := app.UpdateBinary(callCtx, clientApp.UpdateBinaryInput{
		RecordID:        record.RecordID,
		ExpectedVersion: record.Version,
		Title:           title,
		Description:     description,
		Filename:        filename,
		ContentType:     contentType,
		File:            file,
	})
	if err != nil {
		return err
	}
	record.Title = title
	record.Description = description
	record.Filename = filename
	record.ContentType = contentType
	record.Size = int64(len(file))
	record.Version = updated.Version
	state.storeBinary(record)
	fmt.Fprintf(out, "заменен файл бинарной приватной записи: %s\n", record.Title)
	return nil
}

func cachedCredential(
	ctx context.Context,
	app *clientApp.App,
	state *recordsMenuState,
	item clientApp.RecordListItem,
) (clientApp.CredentialRecord, error) {
	if entry, ok := state.cache[item.RecordID]; ok && entry.credential != nil {
		return *entry.credential, nil
	}
	record, err := loadCredential(ctx, app, item.RecordID)
	if err != nil {
		return clientApp.CredentialRecord{}, err
	}
	state.storeCredential(record)
	return record, nil
}

func cachedText(
	ctx context.Context,
	app *clientApp.App,
	state *recordsMenuState,
	item clientApp.RecordListItem,
) (clientApp.TextRecord, error) {
	if entry, ok := state.cache[item.RecordID]; ok && entry.text != nil {
		return *entry.text, nil
	}
	record, err := loadText(ctx, app, item.RecordID)
	if err != nil {
		return clientApp.TextRecord{}, err
	}
	state.storeText(record)
	return record, nil
}

func cachedCard(
	ctx context.Context,
	app *clientApp.App,
	state *recordsMenuState,
	item clientApp.RecordListItem,
) (clientApp.CardRecord, error) {
	if entry, ok := state.cache[item.RecordID]; ok && entry.card != nil {
		return *entry.card, nil
	}
	record, err := loadCard(ctx, app, item.RecordID)
	if err != nil {
		return clientApp.CardRecord{}, err
	}
	state.storeCard(record)
	return record, nil
}

func cachedBinary(
	ctx context.Context,
	app *clientApp.App,
	state *recordsMenuState,
	item clientApp.RecordListItem,
) (clientApp.BinaryRecord, error) {
	if entry, ok := state.cache[item.RecordID]; ok && entry.binary != nil {
		return *entry.binary, nil
	}
	record, err := loadBinary(ctx, app, item.RecordID)
	if err != nil {
		return clientApp.BinaryRecord{}, err
	}
	state.storeBinary(record)
	return record, nil
}

func loadCredential(ctx context.Context, app *clientApp.App, recordID string) (clientApp.CredentialRecord, error) {
	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	return app.GetCredential(callCtx, recordID)
}

func loadText(ctx context.Context, app *clientApp.App, recordID string) (clientApp.TextRecord, error) {
	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	return app.GetText(callCtx, recordID)
}

func loadCard(ctx context.Context, app *clientApp.App, recordID string) (clientApp.CardRecord, error) {
	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	return app.GetCard(callCtx, recordID)
}

func loadBinary(ctx context.Context, app *clientApp.App, recordID string) (clientApp.BinaryRecord, error) {
	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	return app.GetBinary(callCtx, recordID)
}

func printCredentialRecord(out io.Writer, record clientApp.CredentialRecord) {
	fmt.Fprintf(
		out,
		"* тип: credential\n* название: %s\n* описание: %s\n* логин сохраненной учетной записи: %s\n* пароль сохраненной учетной записи: %s\n",
		record.Title,
		record.Description,
		record.Login,
		record.Password,
	)
}

func printTextRecord(out io.Writer, record clientApp.TextRecord) {
	fmt.Fprintf(out, "* тип: text\n* название: %s\n* описание: %s\n* текст: %s\n", record.Title, record.Description, record.Text)
}

func printCardRecord(out io.Writer, record clientApp.CardRecord) {
	fmt.Fprintf(
		out,
		"* тип: card\n* название: %s\n* описание: %s\n* номер карты: %s\n* имя владельца: %s\n* срок действия: %s\n* CVC: %s\n",
		record.Title,
		record.Description,
		record.Number,
		record.HolderName,
		record.ExpiresAt,
		record.CVC,
	)
}

func printBinaryRecord(out io.Writer, record clientApp.BinaryRecord) {
	fmt.Fprintf(
		out,
		"* тип: binary\n* название: %s\n* описание: %s\n* исходное имя: %s\n* MIME-тип: %s\n* размер: %d байт\n",
		record.Title,
		record.Description,
		record.Filename,
		record.ContentType,
		record.Size,
	)
}

func promptWithDefault(reader *bufio.Reader, out io.Writer, label string, current string) (string, error) {
	value, err := prompt(reader, out, fmt.Sprintf("%s [%s]: ", label, current))
	if err != nil {
		return "", err
	}
	if value == "" {
		return current, nil
	}
	return value, nil
}

func waitForEnter(reader *bufio.Reader, out io.Writer) error {
	_, err := prompt(reader, out, "Нажмите Enter, чтобы продолжить...")
	return err
}

func clearScreen(out io.Writer) {
	file, ok := out.(*os.File)
	if !ok || !isTerminal(file) {
		return
	}
	fmt.Fprint(out, "\033[H\033[2J")
}

// createCredential запрашивает поля учетных данных и создает зашифрованную приватную запись.
func createCredential(ctx context.Context, app *clientApp.App, reader *bufio.Reader, out io.Writer) error {
	title, err := promptRequired(reader, out, "Название: ")
	if err != nil {
		return err
	}
	description, err := prompt(reader, out, "Описание: ")
	if err != nil {
		return err
	}
	credentialLogin, err := promptRequired(reader, out, "Логин сохраняемой учетной записи: ")
	if err != nil {
		return err
	}
	credentialPassword, err := promptRequired(reader, out, "Пароль сохраняемой учетной записи: ")
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

// getCredential получает приватную запись с учетными данными и печатает расшифрованный payload.
func getCredential(ctx context.Context, app *clientApp.App, reader *bufio.Reader, out io.Writer) error {
	recordID, err := promptRequired(reader, out, "ID приватной записи: ")
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	record, err := app.GetCredential(callCtx, recordID)
	if err != nil {
		return err
	}
	fmt.Fprintf(
		out,
		"record_id: %s\nверсия: %d\nназвание: %s\nописание: %s\nлогин сохраненной учетной записи: %s\nпароль сохраненной учетной записи: %s\n",
		record.RecordID,
		record.Version,
		record.Title,
		record.Description,
		record.Login,
		record.Password,
	)
	return nil
}

// updateCredential запрашивает новые поля учетных данных и обновляет зашифрованную приватную запись.
func updateCredential(ctx context.Context, app *clientApp.App, reader *bufio.Reader, out io.Writer) error {
	recordID, err := promptRequired(reader, out, "ID приватной записи: ")
	if err != nil {
		return err
	}
	expectedVersion, err := promptExpectedVersion(reader, out)
	if err != nil {
		return err
	}
	title, err := promptRequired(reader, out, "Новое название: ")
	if err != nil {
		return err
	}
	description, err := prompt(reader, out, "Новое описание: ")
	if err != nil {
		return err
	}
	credentialLogin, err := promptRequired(reader, out, "Новый логин обновляемой учетной записи: ")
	if err != nil {
		return err
	}
	credentialPassword, err := promptRequired(reader, out, "Новый пароль обновляемой учетной записи: ")
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	updated, err := app.UpdateCredential(callCtx, clientApp.UpdateCredentialInput{
		RecordID:           recordID,
		ExpectedVersion:    expectedVersion,
		Title:              title,
		Description:        description,
		CredentialLogin:    credentialLogin,
		CredentialPassword: credentialPassword,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(
		out,
		"обновлена приватная запись с учетными данными: record_id=%s version=%d\n",
		updated.RecordID,
		updated.Version,
	)
	return nil
}

// createText запрашивает поля текстовой записи и создает зашифрованную приватную запись.
func createText(ctx context.Context, app *clientApp.App, reader *bufio.Reader, out io.Writer) error {
	title, err := promptRequired(reader, out, "Название: ")
	if err != nil {
		return err
	}
	description, err := prompt(reader, out, "Описание: ")
	if err != nil {
		return err
	}
	text, err := promptRequired(reader, out, "Текст: ")
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

// getText получает текстовую приватную запись и печатает расшифрованный payload.
func getText(ctx context.Context, app *clientApp.App, reader *bufio.Reader, out io.Writer) error {
	recordID, err := promptRequired(reader, out, "ID приватной записи: ")
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	record, err := app.GetText(callCtx, recordID)
	if err != nil {
		return err
	}
	fmt.Fprintf(
		out,
		"record_id: %s\nверсия: %d\nназвание: %s\nописание: %s\nтекст: %s\n",
		record.RecordID,
		record.Version,
		record.Title,
		record.Description,
		record.Text,
	)
	return nil
}

// updateText запрашивает новые поля текстовой записи и обновляет зашифрованную приватную запись.
func updateText(ctx context.Context, app *clientApp.App, reader *bufio.Reader, out io.Writer) error {
	recordID, err := promptRequired(reader, out, "ID приватной записи: ")
	if err != nil {
		return err
	}
	expectedVersion, err := promptExpectedVersion(reader, out)
	if err != nil {
		return err
	}
	title, err := promptRequired(reader, out, "Новое название: ")
	if err != nil {
		return err
	}
	description, err := prompt(reader, out, "Новое описание: ")
	if err != nil {
		return err
	}
	text, err := promptRequired(reader, out, "Новый текст: ")
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	updated, err := app.UpdateText(callCtx, clientApp.UpdateTextInput{
		RecordID:        recordID,
		ExpectedVersion: expectedVersion,
		Title:           title,
		Description:     description,
		Text:            text,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(
		out,
		"обновлена текстовая приватная запись: record_id=%s version=%d\n",
		updated.RecordID,
		updated.Version,
	)
	return nil
}

// createCard запрашивает поля банковской карты и создает зашифрованную приватную запись.
func createCard(ctx context.Context, app *clientApp.App, reader *bufio.Reader, out io.Writer) error {
	title, err := promptRequired(reader, out, "Название: ")
	if err != nil {
		return err
	}
	description, err := prompt(reader, out, "Описание: ")
	if err != nil {
		return err
	}
	number, err := promptRequired(reader, out, "Номер карты: ")
	if err != nil {
		return err
	}
	holderName, err := promptRequired(reader, out, "Имя владельца: ")
	if err != nil {
		return err
	}
	expiresAt, err := promptRequired(reader, out, "Срок действия: ")
	if err != nil {
		return err
	}
	cvc, err := promptRequired(reader, out, "CVC: ")
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

// getCard получает приватную запись банковской карты и печатает расшифрованный payload.
func getCard(ctx context.Context, app *clientApp.App, reader *bufio.Reader, out io.Writer) error {
	recordID, err := promptRequired(reader, out, "ID приватной записи: ")
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	record, err := app.GetCard(callCtx, recordID)
	if err != nil {
		return err
	}
	fmt.Fprintf(
		out,
		"record_id: %s\nверсия: %d\nназвание: %s\nописание: %s\nномер карты: %s\nимя владельца: %s\nсрок действия: %s\nCVC: %s\n",
		record.RecordID,
		record.Version,
		record.Title,
		record.Description,
		record.Number,
		record.HolderName,
		record.ExpiresAt,
		record.CVC,
	)
	return nil
}

// updateCard запрашивает новые поля банковской карты и обновляет зашифрованную приватную запись.
func updateCard(ctx context.Context, app *clientApp.App, reader *bufio.Reader, out io.Writer) error {
	recordID, err := promptRequired(reader, out, "ID приватной записи: ")
	if err != nil {
		return err
	}
	expectedVersion, err := promptExpectedVersion(reader, out)
	if err != nil {
		return err
	}
	title, err := promptRequired(reader, out, "Новое название: ")
	if err != nil {
		return err
	}
	description, err := prompt(reader, out, "Новое описание: ")
	if err != nil {
		return err
	}
	number, err := promptRequired(reader, out, "Новый номер карты: ")
	if err != nil {
		return err
	}
	holderName, err := promptRequired(reader, out, "Новое имя владельца: ")
	if err != nil {
		return err
	}
	expiresAt, err := promptRequired(reader, out, "Новый срок действия: ")
	if err != nil {
		return err
	}
	cvc, err := promptRequired(reader, out, "Новый CVC: ")
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	updated, err := app.UpdateCard(callCtx, clientApp.UpdateCardInput{
		RecordID:        recordID,
		ExpectedVersion: expectedVersion,
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
	fmt.Fprintf(
		out,
		"обновлена приватная запись банковской карты: record_id=%s version=%d\n",
		updated.RecordID,
		updated.Version,
	)
	return nil
}

// createBinary запрашивает метаданные и путь к файлу, затем создает зашифрованную файловую приватную запись.
func createBinary(ctx context.Context, app *clientApp.App, reader *bufio.Reader, out io.Writer) error {
	title, err := promptRequired(reader, out, "Название: ")
	if err != nil {
		return err
	}
	description, err := prompt(reader, out, "Описание: ")
	if err != nil {
		return err
	}
	path, err := promptRequiredRetry(reader, out, "Путь к файлу: ", "путь к файлу обязателен")
	if err != nil {
		return err
	}

	file, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("не удалось прочитать файл: %w", err)
	}

	filename := filepath.Base(path)
	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	_, err = app.CreateBinary(callCtx, clientApp.CreateBinaryInput{
		Title:       title,
		Description: description,
		Filename:    filename,
		ContentType: contentType,
		File:        file,
	})
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "создана файловая приватная запись")
	return nil
}

// updateBinaryMetadata запрашивает новые открытые метаданные и обновляет бинарную приватную запись без замены файла.
func updateBinaryMetadata(ctx context.Context, app *clientApp.App, reader *bufio.Reader, out io.Writer) error {
	recordID, err := promptRequired(reader, out, "ID приватной записи: ")
	if err != nil {
		return err
	}
	expectedVersion, err := promptExpectedVersion(reader, out)
	if err != nil {
		return err
	}
	title, err := promptRequired(reader, out, "Новое название: ")
	if err != nil {
		return err
	}
	description, err := prompt(reader, out, "Новое описание: ")
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	updated, err := app.UpdateBinaryMetadata(callCtx, clientApp.UpdateBinaryMetadataInput{
		RecordID:        recordID,
		ExpectedVersion: expectedVersion,
		Title:           title,
		Description:     description,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(
		out,
		"обновлены метаданные бинарной приватной записи: record_id=%s version=%d\n",
		updated.RecordID,
		updated.Version,
	)
	return nil
}

// replaceBinaryFile запрашивает новые метаданные и путь к файлу, затем заменяет файл бинарной приватной записи.
func replaceBinaryFile(ctx context.Context, app *clientApp.App, reader *bufio.Reader, out io.Writer) error {
	recordID, err := promptRequired(reader, out, "ID приватной записи: ")
	if err != nil {
		return err
	}
	expectedVersion, err := promptExpectedVersion(reader, out)
	if err != nil {
		return err
	}
	title, err := promptRequired(reader, out, "Новое название: ")
	if err != nil {
		return err
	}
	description, err := prompt(reader, out, "Новое описание: ")
	if err != nil {
		return err
	}
	path, err := promptRequired(reader, out, "Путь к новому файлу: ")
	if err != nil {
		return err
	}

	file, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("не удалось прочитать файл: %w", err)
	}

	filename := filepath.Base(path)
	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	updated, err := app.UpdateBinary(callCtx, clientApp.UpdateBinaryInput{
		RecordID:        recordID,
		ExpectedVersion: expectedVersion,
		Title:           title,
		Description:     description,
		Filename:        filename,
		ContentType:     contentType,
		File:            file,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(
		out,
		"заменен файл бинарной приватной записи: record_id=%s version=%d\n",
		updated.RecordID,
		updated.Version,
	)
	return nil
}

// downloadBinaryFile скачивает файл приватной записи, расшифровывает его и сохраняет по указанному пути.
func downloadBinaryFile(ctx context.Context, app *clientApp.App, reader *bufio.Reader, out io.Writer) error {
	recordID, err := promptRequired(reader, out, "ID приватной записи: ")
	if err != nil {
		return err
	}
	outputPath, err := promptRequired(reader, out, "Путь для сохранения файла: ")
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	file, err := app.DownloadBinaryFile(callCtx, recordID)
	if err != nil {
		return err
	}

	if err = os.WriteFile(outputPath, file.Data, 0o600); err != nil {
		return fmt.Errorf("не удалось сохранить файл: %w", err)
	}
	fmt.Fprintf(
		out,
		"файл сохранен: %s\nrecord_id: %s\nисходное имя: %s\nMIME-тип: %s\nразмер: %d байт\n",
		outputPath,
		file.RecordID,
		file.Filename,
		file.ContentType,
		file.DeclaredSize,
	)
	return nil
}

// listRecords получает и печатает список приватных записей пользователя.
func listRecords(ctx context.Context, app *clientApp.App, out io.Writer) error {
	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	items, err := app.ListRecords(callCtx)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Fprintln(out, "приватные записи не найдены")
		return nil
	}
	for _, item := range items {
		fmt.Fprintf(out, "%s | %s | %s | %s\n", item.RecordID, item.Type, item.Title, item.Description)
	}
	return nil
}

// deleteRecord удаляет приватную запись пользователя.
func deleteRecord(ctx context.Context, app *clientApp.App, reader *bufio.Reader, out io.Writer) error {
	recordID, err := promptRequired(reader, out, "ID приватной записи: ")
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	deleted, err := app.DeleteRecord(callCtx, recordID)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "приватная запись удалена: record_id=%s\n", deleted.RecordID)
	return nil
}

// promptExpectedVersion запрашивает версию приватной записи, на основе которой выполняется обновление.
func promptExpectedVersion(reader *bufio.Reader, out io.Writer) (int64, error) {
	value, err := promptRequired(reader, out, "Текущая версия: ")
	if err != nil {
		return 0, err
	}
	version, err := strconv.ParseInt(value, 10, 64)
	if err != nil || version <= 0 {
		return 0, errors.New("текущая версия должна быть положительным целым числом")
	}
	return version, nil
}

// logout завершает пользовательскую сессию и возвращает клиента в стартовое меню.
func logout(ctx context.Context, app *clientApp.App, out io.Writer) error {
	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	if err := app.Logout(callCtx); err != nil {
		return err
	}
	fmt.Fprintln(out, "выполнен выход из аккаунта")
	return nil
}

// logoutAndExit пытается завершить пользовательскую сессию перед выходом из приложения.
func logoutAndExit(ctx context.Context, app *clientApp.App, out io.Writer) error {
	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	if err := app.Logout(callCtx); err != nil {
		printError(out, err)
	}
	return nil
}

// printError печатает пользовательскую ошибку в едином формате CLI-клиента.
func printError(out io.Writer, err error) {
	fmt.Fprintf(out, "ошибка: %v\n", err)
}
