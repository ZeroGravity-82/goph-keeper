package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// confirm запрашивает подтверждение действия в формате y/N.
func confirm(reader *bufio.Reader, out io.Writer, question string) (bool, error) {
	answer, err := prompt(reader, out, question+" [y/N]: ")
	if err != nil {
		return false, err
	}
	return strings.ToLower(answer) == "y", nil
}

// printActionError печатает отмену действия как штатный результат, а остальные ошибки в общем формате.
func printActionError(out io.Writer, err error) {
	if errors.Is(err, errActionCanceled) {
		fmt.Fprintln(out, "действие отменено")
		return
	}
	printError(out, err)
}

// promptWithDefault запрашивает значение и возвращает текущее значение при пустом вводе.
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

// promptWithDefaultCancelable запрашивает значение по умолчанию с поддержкой отмены действия.
func promptWithDefaultCancelable(reader *bufio.Reader, out io.Writer, label string, current string) (string, error) {
	value, err := promptCancelable(reader, out, fmt.Sprintf("%s [%s]: ", label, current))
	if err != nil {
		return "", err
	}
	if value == "" {
		return current, nil
	}
	return value, nil
}

// printCancelHint печатает подсказку с командами отмены текущего действия.
func printCancelHint(out io.Writer) {
	fmt.Fprintln(out, "Чтобы отменить действие, введите :q, cancel или отмена.")
}

// waitForEnter ожидает подтверждающее нажатие Enter перед продолжением.
func waitForEnter(reader *bufio.Reader, out io.Writer) error {
	_, err := prompt(reader, out, "Нажмите Enter, чтобы продолжить...")
	return err
}

// pauseBeforeClearScreen делает короткую паузу перед очисткой интерактивного терминала.
func pauseBeforeClearScreen(out io.Writer) {
	file, ok := out.(*os.File)
	if !ok || !isTerminal(file) {
		return
	}
	time.Sleep(time.Second)
}

// clearScreen очищает экран ANSI-последовательностью.
func clearScreen(out io.Writer) {
	fmt.Fprint(out, "\033[H\033[2J")
}

// printError печатает пользовательскую ошибку в едином формате CLI-клиента.
func printError(out io.Writer, err error) {
	fmt.Fprintf(out, "ошибка: %v\n", err)
}
