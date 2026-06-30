package client

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func rpcError(err error, fallback string, messages map[codes.Code]string) error {
	code := status.Code(err)
	if message, ok := messages[code]; ok {
		return errors.New(message)
	}
	switch code {
	case codes.DeadlineExceeded:
		return errors.New("сервер не ответил вовремя")
	case codes.Unavailable:
		return errors.New("сервер временно недоступен")
	case codes.Internal:
		return errors.New("внутренняя ошибка сервера")
	default:
		return fmt.Errorf("%s: %w", fallback, err)
	}
}
