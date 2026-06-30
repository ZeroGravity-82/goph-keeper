package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

// promptSecret запрашивает секретное значение и скрывает ввод, если stdin является терминалом.
//
// Если ввод идет не из терминала, функция использует обычный prompt. Это упрощает тестирование и позволяет запускать
// клиент с перенаправленным вводом.
func promptSecret(reader *bufio.Reader, in io.Reader, out io.Writer, label string) (string, error) {
	if file, ok := in.(*os.File); ok && isTerminal(file) && reader.Buffered() == 0 {
		fmt.Fprint(out, label)
		value, err := readSecretFromTerminal(file)
		fmt.Fprintln(out)
		if err != nil {
			return "", err
		}
		return normalizeInput(value)
	}
	return prompt(reader, out, label)
}

// promptSecretConfirmed запрашивает секретное значение дважды и проверяет совпадение ввода.
func promptSecretConfirmed(
	reader *bufio.Reader,
	in io.Reader,
	out io.Writer,
	label string,
	confirmLabel string,
) (string, error) {
	value, err := promptSecret(reader, in, out, label)
	if err != nil {
		return "", err
	}
	confirmed, err := promptSecret(reader, in, out, confirmLabel)
	if err != nil {
		return "", err
	}
	if value != confirmed {
		return "", errors.New("значения не совпадают")
	}
	return value, nil
}

// isTerminal проверяет, что файл связан с интерактивным терминалом.
func isTerminal(file *os.File) bool {
	_, err := unix.IoctlGetTermios(int(file.Fd()), unix.TCGETS)
	return err == nil
}

// readSecretFromTerminal читает строку из терминала с временно отключенным отображением ввода.
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
	defer unix.IoctlSetTermios(fd, unix.TCSETS, oldState)

	value, err := readLineFromFile(file)
	if err != nil {
		return "", err
	}
	return value, nil
}

// readLineFromFile читает строку из файла до перевода строки, возврата каретки или EOF.
func readLineFromFile(file *os.File) (string, error) {
	var buffer bytes.Buffer
	chunk := make([]byte, 1)
	for {
		n, err := file.Read(chunk)
		if n > 0 {
			if chunk[0] == '\n' || chunk[0] == '\r' {
				return buffer.String(), nil
			}
			buffer.WriteByte(chunk[0])
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return buffer.String(), nil
			}
			return "", fmt.Errorf("не удалось прочитать ввод: %w", err)
		}
	}
}

// promptRequired запрашивает обязательное значение и возвращает ошибку при пустом вводе.
func promptRequired(reader *bufio.Reader, out io.Writer, label string) (string, error) {
	value, err := prompt(reader, out, label)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", fmt.Errorf("%s обязателен", strings.TrimSuffix(label, ": "))
	}
	return value, nil
}

// prompt печатает приглашение, читает одну строку пользовательского ввода и нормализует ее.
func prompt(reader *bufio.Reader, out io.Writer, label string) (string, error) {
	fmt.Fprint(out, label)
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("не удалось прочитать ввод: %w", err)
	}
	return normalizeInput(value)
}

// normalizeInput удаляет пробельные символы по краям и проверяет корректность UTF-8.
func normalizeInput(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) {
		return "", errors.New("ввод должен быть корректной UTF-8 строкой")
	}
	return value, nil
}
