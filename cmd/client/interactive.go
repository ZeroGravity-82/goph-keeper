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

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
)

// runStartMenu запускает стартовое меню CLI-клиента до регистрации, входа в аккаунт или выхода из приложения.
func runStartMenu(ctx context.Context, app *clientApp.App, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	for {
		printStartMenu(out)
		choice, err := promptRequired(reader, out, "Выберите действие: ")
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
	for {
		printRecordsMenu(out)
		choice, err := promptRequired(reader, out, "Выберите действие: ")
		if err != nil {
			return err
		}

		switch choice {
		case "1":
			if err := createCredential(ctx, app, reader, out); err != nil {
				printError(out, err)
			}
		case "2":
			if err := getCredential(ctx, app, reader, out); err != nil {
				printError(out, err)
			}
		case "3":
			if err := createText(ctx, app, reader, out); err != nil {
				printError(out, err)
			}
		case "4":
			if err := getText(ctx, app, reader, out); err != nil {
				printError(out, err)
			}
		case "5":
			if err := createCard(ctx, app, reader, out); err != nil {
				printError(out, err)
			}
		case "6":
			if err := getCard(ctx, app, reader, out); err != nil {
				printError(out, err)
			}
		case "7":
			if err := createBinary(ctx, app, reader, out); err != nil {
				printError(out, err)
			}
		case "8":
			if err := downloadBinaryFile(ctx, app, reader, out); err != nil {
				printError(out, err)
			}
		case "9":
			if err := listRecords(ctx, app, out); err != nil {
				printError(out, err)
			}
		case "10":
			if err := logout(ctx, app, out); err != nil {
				printError(out, err)
			}
			return nil
		case "11":
			return logoutAndExit(ctx, app, out)
		default:
			fmt.Fprintln(out, "неизвестное действие")
		}
	}
}

// printRecordsMenu печатает меню действий, доступных после успешного входа в аккаунт.
func printRecordsMenu(out io.Writer) {
	_, _ = fmt.Fprintln(out, `
1. Создать учетные данные
2. Получить учетные данные
3. Создать текстовую запись
4. Получить текстовую запись
5. Создать банковскую карту
6. Получить банковскую карту
7. Создать файловую запись
8. Скачать файл
9. Показать список приватных записей
10. Выйти из аккаунта
11. Завершить приложение`)
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
	credentialLogin, err := promptRequired(reader, out, "Логин учетной записи: ")
	if err != nil {
		return err
	}
	credentialPassword, err := promptRequired(reader, out, "Пароль учетной записи: ")
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	created, err := app.CreateCredential(callCtx, clientApp.CreateCredentialInput{
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
		"создана приватная запись с учетными данными: record_id=%s version=%d\n",
		created.RecordID,
		created.Version,
	)
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
		"record_id: %s\nназвание: %s\nописание: %s\nлогин учетной записи: %s\nпароль учетной записи: %s\n",
		record.RecordID,
		record.Title,
		record.Description,
		record.Login,
		record.Password,
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
	created, err := app.CreateText(callCtx, clientApp.CreateTextInput{
		Title:       title,
		Description: description,
		Text:        text,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(
		out,
		"создана текстовая приватная запись: record_id=%s version=%d\n",
		created.RecordID,
		created.Version,
	)
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
		"record_id: %s\nназвание: %s\nописание: %s\nтекст: %s\n",
		record.RecordID,
		record.Title,
		record.Description,
		record.Text,
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
	created, err := app.CreateCard(callCtx, clientApp.CreateCardInput{
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
	fmt.Fprintf(
		out,
		"создана приватная запись банковской карты: record_id=%s version=%d\n",
		created.RecordID,
		created.Version,
	)
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
		"record_id: %s\nназвание: %s\nописание: %s\nномер карты: %s\nимя владельца: %s\nсрок действия: %s\nCVC: %s\n",
		record.RecordID,
		record.Title,
		record.Description,
		record.Number,
		record.HolderName,
		record.ExpiresAt,
		record.CVC,
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
	path, err := promptRequired(reader, out, "Путь к файлу: ")
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
	created, err := app.CreateBinary(callCtx, clientApp.CreateBinaryInput{
		Title:       title,
		Description: description,
		Filename:    filename,
		ContentType: contentType,
		File:        file,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(
		out,
		"создана файловая приватная запись: record_id=%s version=%d\n",
		created.RecordID,
		created.Version,
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
