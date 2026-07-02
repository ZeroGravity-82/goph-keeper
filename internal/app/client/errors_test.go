package client

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
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
