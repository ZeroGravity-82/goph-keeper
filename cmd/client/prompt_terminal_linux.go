//go:build linux

package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// isTerminal проверяет, что файл связан с интерактивным терминалом Linux.
//
// unix.IoctlGetTermios читает настройки терминала по файловому дескриптору. Если дескриптор указывает на обычный файл,
// пайп или перенаправленный ввод, то у него нет атрибутов терминала и вызов вернет ошибку. Поэтому успешный вызов здесь
// используется как простой признак настоящего TTY.
func isTerminal(file *os.File) bool {
	_, err := unix.IoctlGetTermios(int(file.Fd()), unix.TCGETS)
	return err == nil
}

// readSecretFromTerminal читает строку из терминала Linux со временно отключенным отображением ввода.
//
// Это нужно для секретных значений вроде пароля и мастер-ключа: при интерактивном вводе они не должны отображаться на
// экране и оставаться в истории вывода терминала.
func readSecretFromTerminal(file *os.File) (string, error) {
	fd := int(file.Fd())
	oldState, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать настройки терминала: %w", err)
	}
	newState := *oldState
	newState.Lflag &^= unix.ECHO
	if err = unix.IoctlSetTermios(fd, unix.TCSETS, &newState); err != nil {
		return "", fmt.Errorf("не удалось отключить отображение ввода в терминале: %w", err)
	}
	defer func() {
		_ = unix.IoctlSetTermios(fd, unix.TCSETS, oldState)
	}()

	return readLineFromFile(file)
}
