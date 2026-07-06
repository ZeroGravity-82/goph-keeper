package client

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// ErrServerTimeout возвращается, когда сервер не ответил в рамках клиентского deadline.
	ErrServerTimeout = errors.New("сервер не ответил вовремя")
	// ErrServerUnavailable возвращается, когда сервер временно недоступен.
	ErrServerUnavailable = errors.New("сервер временно недоступен")
)

// IsConnectionError проверяет, что ошибка связана с потерей связи с сервером или истечением deadline запроса.
func IsConnectionError(err error) bool {
	return errors.Is(err, ErrServerTimeout) || errors.Is(err, ErrServerUnavailable)
}

func rpcError(err error, fallback string, messages map[codes.Code]string) error {
	code := status.Code(err)
	if code == codes.Unknown {
		return err
	}
	if message, ok := messages[code]; ok {
		return rpcMessageError(code, message)
	}
	switch code {
	case codes.DeadlineExceeded:
		return ErrServerTimeout
	case codes.Unavailable:
		return ErrServerUnavailable
	case codes.Internal:
		return errors.New("внутренняя ошибка сервера")
	default:
		return fmt.Errorf("%s: %w", fallback, err)
	}
}

func rpcMessageError(code codes.Code, message string) error {
	switch code {
	case codes.DeadlineExceeded:
		return ErrServerTimeout
	case codes.Unavailable:
		return ErrServerUnavailable
	default:
		return errors.New(message)
	}
}
