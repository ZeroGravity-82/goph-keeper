//go:build windows

package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// isTerminal проверяет, что файл связан с интерактивной консолью Windows.
//
// windows.GetConsoleMode читает режим консольного дескриптора. Для обычного файла, пайпа или перенаправленного ввода
// этот вызов возвращает ошибку, поэтому успешный вызов используется как признак настоящей консоли.
func isTerminal(file *os.File) bool {
	var mode uint32
	err := windows.GetConsoleMode(windows.Handle(file.Fd()), &mode)
	return err == nil
}

// readSecretFromTerminal читает строку из консоли Windows со временно отключенным отображением ввода.
//
// Это нужно для секретных значений вроде пароля и мастер-ключа: при интерактивном вводе они не должны отображаться на
// экране и оставаться в истории вывода терминала.
func readSecretFromTerminal(file *os.File) (string, error) {
	handle := windows.Handle(file.Fd())
	var oldMode uint32
	if err := windows.GetConsoleMode(handle, &oldMode); err != nil {
		return "", fmt.Errorf("не удалось прочитать настройки терминала: %w", err)
	}
	newMode := oldMode &^ windows.ENABLE_ECHO_INPUT
	if err := windows.SetConsoleMode(handle, newMode); err != nil {
		return "", fmt.Errorf("не удалось отключить отображение ввода в терминале: %w", err)
	}
	defer windows.SetConsoleMode(handle, oldMode)

	return readLineFromFile(file)
}
