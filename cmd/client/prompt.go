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

// requiredInputError обозначает пустой ввод обязательного поля.
//
// Отдельный тип ошибки позволяет promptRequiredRetry отличать ошибку валидации обязательного поля от ошибок чтения
// ввода и повторять запрос только в этом случае.
type requiredInputError struct {
	message string
}

// Error возвращает текст ошибки обязательного поля.
func (e requiredInputError) Error() string {
	return e.message
}

// promptSecret запрашивает секретное значение и скрывает ввод, если stdin является терминалом.
//
// Если ввод идет не из терминала, функция использует обычный prompt. Это упрощает тестирование и позволяет запускать
// клиента с перенаправленным вводом.
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
//
// Это используется при задании нового секрета, чтобы пользователь не сохранил пароль или мастер-ключ с незамеченной
// опечаткой.
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
//
// unix.IoctlGetTermios читает настройки терминала по файловому дескриптору. Если дескриптор указывает на обычный файл,
// пайп или перенаправленный ввод, то у него нет атрибутов терминала и вызов вернет ошибку. Поэтому успешный вызов здесь
// используется как простой признак настоящего TTY.
func isTerminal(file *os.File) bool {
	_, err := unix.IoctlGetTermios(int(file.Fd()), unix.TCGETS)
	return err == nil
}

// readSecretFromTerminal читает строку из терминала со временно отключенным отображением ввода.
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
	return promptRequiredNamed(reader, out, label, strings.TrimSuffix(label, ": "))
}

// promptRequiredNamed запрашивает обязательное значение и использует fieldName в сообщении об ошибке.
func promptRequiredNamed(reader *bufio.Reader, out io.Writer, label string, fieldName string) (string, error) {
	return promptRequiredWithError(reader, out, label, fmt.Sprintf("%s обязателен", fieldName))
}

// promptRequiredWithError запрашивает обязательное значение и возвращает requiredError при пустом вводе.
func promptRequiredWithError(reader *bufio.Reader, out io.Writer, label string, requiredError string) (string, error) {
	value, err := prompt(reader, out, label)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", requiredInputError{message: requiredError}
	}
	return value, nil
}

// promptRequiredRetry повторяет ввод обязательного поля, если пользователь оставил его пустым.
func promptRequiredRetry(reader *bufio.Reader, out io.Writer, label string, requiredError string) (string, error) {
	for {
		value, err := promptRequiredWithError(reader, out, label, requiredError)
		if err == nil {
			return value, nil
		}
		var requiredErr requiredInputError
		if !errors.As(err, &requiredErr) {
			return "", err
		}
		printError(out, err)
	}
}

// promptCancelable запрашивает значение и возвращает errActionCanceled при вводе команды отмены.
func promptCancelable(reader *bufio.Reader, out io.Writer, label string) (string, error) {
	value, err := prompt(reader, out, label)
	if err != nil {
		return "", err
	}
	if isCancelInput(value) {
		return "", errActionCanceled
	}
	return value, nil
}

// promptRequiredCancelable запрашивает обязательное значение с поддержкой команды отмены.
func promptRequiredCancelable(reader *bufio.Reader, out io.Writer, label string) (string, error) {
	return promptRequiredWithErrorCancelable(reader, out, label, fmt.Sprintf(
		"%s обязателен",
		strings.TrimSuffix(label, ": "),
	))
}

// promptRequiredWithErrorCancelable запрашивает обязательное значение с заданным текстом ошибки и поддержкой команды
// отмены.
func promptRequiredWithErrorCancelable(
	reader *bufio.Reader,
	out io.Writer,
	label string,
	requiredError string,
) (string, error) {
	value, err := promptCancelable(reader, out, label)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", requiredInputError{message: requiredError}
	}
	return value, nil
}

// promptRequiredRetryCancelable повторяет запрос обязательного значения, но сразу завершает действие по команде отмены.
func promptRequiredRetryCancelable(
	reader *bufio.Reader,
	out io.Writer,
	label string,
	requiredError string,
) (string, error) {
	for {
		value, err := promptRequiredWithErrorCancelable(reader, out, label, requiredError)
		if err == nil {
			return value, nil
		}
		if errors.Is(err, errActionCanceled) {
			return "", err
		}
		var requiredErr requiredInputError
		if !errors.As(err, &requiredErr) {
			return "", err
		}
		printError(out, err)
	}
}

// prompt печатает промпт (приглашение к вводу), читает одну строку пользовательского ввода и нормализует ее.
func prompt(reader *bufio.Reader, out io.Writer, label string) (string, error) {
	fmt.Fprint(out, label)
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("не удалось прочитать ввод: %w", err)
	}
	return normalizeInput(value)
}

// isCancelInput проверяет, что пользователь ввел команду отмены текущего действия.
func isCancelInput(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ":q", "cancel", "отмена":
		return true
	default:
		return false
	}
}

// normalizeInput применяет управляющие символы терминального ввода, удаляет пробельные символы по краям и проверяет
// корректность UTF-8.
func normalizeInput(value string) (string, error) {
	value = applyTerminalControls(value)
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) {
		return "", errors.New("ввод должен быть корректной UTF-8 строкой")
	}
	return value, nil
}

// applyTerminalControls учитывает управляющие байты терминального ввода: Backspace удаляет предыдущий символ,
// ANSI escape-последовательности пропускаются, остальные символы сохраняются.
func applyTerminalControls(value string) string {
	input := []byte(value)
	output := make([]byte, 0, len(input))
	for i := 0; i < len(input); i++ {
		switch input[i] {
		case '\b', 0x7f:
			output = eraseLastInputRune(output)
		case 0x1b:
			i = skipEscapeSequence(input, i)
		default:
			if input[i] < utf8.RuneSelf {
				output = append(output, input[i])
				continue
			}
			r, size := utf8.DecodeRune(input[i:])
			if r == utf8.RuneError && size == 1 {
				output = append(output, input[i])
				continue
			}
			output = append(output, input[i:i+size]...)
			i += size - 1
		}
	}
	return string(dropInvalidUTF8Bytes(output))
}

// eraseLastInputRune удаляет последний введенный символ с учетом UTF-8.
func eraseLastInputRune(value []byte) []byte {
	if len(value) == 0 {
		return value
	}
	_, size := utf8.DecodeLastRune(value)
	if size <= 1 {
		return value[:len(value)-1]
	}
	return value[:len(value)-size]
}

// skipEscapeSequence пропускает ANSI escape-последовательность, начиная с ESC.
func skipEscapeSequence(input []byte, start int) int {
	for i := start + 1; i < len(input); i++ {
		if input[i] >= '@' && input[i] <= '~' {
			return i
		}
	}
	return len(input) - 1
}

// dropInvalidUTF8Bytes удаляет битые байты UTF-8 из терминального ввода.
func dropInvalidUTF8Bytes(input []byte) []byte {
	output := make([]byte, 0, len(input))
	for i := 0; i < len(input); i++ {
		if input[i] < utf8.RuneSelf {
			output = append(output, input[i])
			continue
		}
		r, size := utf8.DecodeRune(input[i:])
		if r == utf8.RuneError && size == 1 {
			continue
		}
		output = append(output, input[i:i+size]...)
		i += size - 1
	}
	return output
}
