package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
)

// runStartMenu запускает стартовое меню CLI-клиента для регистрации, входа в аккаунт или выхода из приложения.
func runStartMenu(ctx context.Context, app *clientApp.App, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	for {
		clearScreen(out)
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
				if err = waitForEnter(reader, out); err != nil {
					return err
				}
				continue
			}
			if loggedIn {
				if err = runRecordsMenu(ctx, app, reader, out); err != nil {
					if errors.Is(err, errExitApplication) {
						return nil
					}
					return err
				}
			}
		case "2":
			loggedIn, err := login(ctx, app, reader, in, out)
			if err != nil {
				printError(out, err)
				if err = waitForEnter(reader, out); err != nil {
					return err
				}
				continue
			}
			if loggedIn {
				if err = runRecordsMenu(ctx, app, reader, out); err != nil {
					if errors.Is(err, errExitApplication) {
						return nil
					}
					return err
				}
			}
		case "3":
			confirmed, err := confirm(reader, out, "Завершить приложение?")
			if err != nil {
				return err
			}
			if !confirmed {
				continue
			}
			return nil
		default:
			fmt.Fprintln(out, "неизвестное действие")
			if err = waitForEnter(reader, out); err != nil {
				return err
			}
		}
	}
}

// printStartMenu печатает меню действий, доступных без активной пользовательской сессии.
func printStartMenu(out io.Writer) {
	fmt.Fprintln(out, "1. Зарегистрироваться")
	fmt.Fprintln(out, "2. Войти в аккаунт")
	fmt.Fprintln(out, "3. Завершить приложение")
}

// login выполняет вход пользователя в аккаунт, проверяет мастер-ключ и открывает пользовательскую сессию.
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

	fmt.Fprintf(out, "выполнен вход: %q\n\n", login)
	printWelcome(out, login)
	return true, nil
}

// printWelcome печатает приветствие после успешной аутентификации.
func printWelcome(out io.Writer, login string) {
	fmt.Fprintf(out, "Добро пожаловать в GophKeeper, %s.\n", login)
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
	return promptSecretConfirmedRequired(
		reader,
		in,
		out,
		"Мастер-ключ: ",
		"Повторите мастер-ключ: ",
		"мастер-ключ обязателен",
	)
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

// logout выполняет выход пользователя из аккаунта, завершает пользовательскую сессию и возвращает клиента в
// стартовое меню.
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
