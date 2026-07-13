package main

import (
	"bufio"
	"io"
	"strings"

	clientApp "zerogravity-82/goph-keeper/internal/app/client"
)

// promptCardNumber запрашивает номер карты и повторяет ввод до корректного значения или отмены.
func promptCardNumber(reader *bufio.Reader, out io.Writer, label string) (string, error) {
	for {
		value, err := promptRequiredRetryCancelable(reader, out, label, "номер карты обязателен")
		if err != nil {
			return "", err
		}
		normalized, err := clientApp.NormalizeCardNumber(value)
		if err != nil {
			printError(out, err)
			continue
		}
		if err = clientApp.ValidateCardNumber(normalized); err != nil {
			printError(out, err)
			continue
		}
		return normalized, nil
	}
}

// promptCardNumberWithDefault запрашивает номер карты с текущим значением по умолчанию.
func promptCardNumberWithDefault(reader *bufio.Reader, out io.Writer, label string, current string) (string, error) {
	for {
		value, err := promptWithDefaultCancelable(reader, out, label, formatCardNumber(current))
		if err != nil {
			return "", err
		}
		normalized, err := clientApp.NormalizeCardNumber(value)
		if err != nil {
			printError(out, err)
			continue
		}
		if err = clientApp.ValidateCardNumber(normalized); err != nil {
			printError(out, err)
			continue
		}
		return normalized, nil
	}
}

// promptCardHolderName запрашивает и нормализует имя владельца карты.
func promptCardHolderName(reader *bufio.Reader, out io.Writer, label string) (string, error) {
	for {
		value, err := promptRequiredRetryCancelable(reader, out, label, "имя владельца карты обязательно")
		if err != nil {
			return "", err
		}
		normalized, err := clientApp.NormalizeCardHolderName(value)
		if err != nil {
			printError(out, err)
			continue
		}
		return normalized, nil
	}
}

// promptCardHolderNameWithDefault запрашивает имя владельца карты с текущим значением по умолчанию.
func promptCardHolderNameWithDefault(
	reader *bufio.Reader,
	out io.Writer,
	label string,
	current string,
) (string, error) {
	for {
		value, err := promptWithDefaultCancelable(reader, out, label, current)
		if err != nil {
			return "", err
		}
		normalized, err := clientApp.NormalizeCardHolderName(value)
		if err != nil {
			printError(out, err)
			continue
		}
		return normalized, nil
	}
}

// promptCardExpiration запрашивает срок действия карты и проверяет формат ММ/ГГ.
func promptCardExpiration(reader *bufio.Reader, out io.Writer, label string) (string, error) {
	for {
		value, err := promptRequiredRetryCancelable(reader, out, label, "срок действия карты обязателен")
		if err != nil {
			return "", err
		}
		if err = clientApp.ValidateCardExpiration(value); err != nil {
			printError(out, err)
			continue
		}
		return strings.TrimSpace(value), nil
	}
}

// promptCardExpirationWithDefault запрашивает срок действия карты с текущим значением по умолчанию.
func promptCardExpirationWithDefault(
	reader *bufio.Reader,
	out io.Writer,
	label string,
	current string,
) (string, error) {
	for {
		value, err := promptWithDefaultCancelable(reader, out, label, current)
		if err != nil {
			return "", err
		}
		if err = clientApp.ValidateCardExpiration(value); err != nil {
			printError(out, err)
			continue
		}
		return strings.TrimSpace(value), nil
	}
}

// promptCardCVC запрашивает CVC и проверяет строгий формат из трех цифр.
func promptCardCVC(reader *bufio.Reader, out io.Writer, label string) (string, error) {
	for {
		value, err := promptRequiredRetryCancelable(reader, out, label, "CVC обязателен")
		if err != nil {
			return "", err
		}
		if err = clientApp.ValidateCardCVC(value); err != nil {
			printError(out, err)
			continue
		}
		return strings.TrimSpace(value), nil
	}
}

// promptCardCVCWithDefault запрашивает CVC с текущим значением по умолчанию.
func promptCardCVCWithDefault(reader *bufio.Reader, out io.Writer, label string, current string) (string, error) {
	for {
		value, err := promptWithDefaultCancelable(reader, out, label, current)
		if err != nil {
			return "", err
		}
		if err = clientApp.ValidateCardCVC(value); err != nil {
			printError(out, err)
			continue
		}
		return strings.TrimSpace(value), nil
	}
}
