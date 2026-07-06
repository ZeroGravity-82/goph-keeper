package client

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Test_rpcError_ReturnsLocalErrorAsIs проверяет, что локальная ошибка клиента не заворачивается в общее сообщение об
// RPC-ошибке.
func Test_rpcError_ReturnsLocalErrorAsIs(t *testing.T) {
	// Arrange
	errLocal := errors.New("сессия истекла, войдите в аккаунт снова")

	// Act
	err := rpcError(errLocal, "не удалось выполнить запрос", nil)

	// Assert
	assert.ErrorIs(t, err, errLocal)
	assert.Equal(t, errLocal.Error(), err.Error())
}

// Test_rpcError_ReturnsConnectionSentinel проверяет, что временная недоступность сервера возвращается как
// распознаваемая ошибка связи.
func Test_rpcError_ReturnsConnectionSentinel(t *testing.T) {
	// Act
	err := rpcError(status.Error(codes.Unavailable, "server is down"), "не удалось выполнить запрос", nil)

	// Assert
	assert.ErrorIs(t, err, ErrServerUnavailable)
	assert.True(t, IsConnectionError(err))
}

// Test_rpcError_ReturnsConnectionSentinelFromCustomMessage проверяет распознаваемую ошибку связи для gRPC-кода из
// таблицы пользовательских сообщений.
func Test_rpcError_ReturnsConnectionSentinelFromCustomMessage(t *testing.T) {
	// Act
	err := rpcError(
		status.Error(codes.Unavailable, "server is down"),
		"не удалось выполнить запрос",
		map[codes.Code]string{codes.Unavailable: "сервер временно недоступен"},
	)

	// Assert
	assert.ErrorIs(t, err, ErrServerUnavailable)
	assert.True(t, IsConnectionError(err))
}
