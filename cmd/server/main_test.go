package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/goph-keeper/internal/config"
	"zerogravity-82/goph-keeper/internal/logging"
)

// Test_run_ReturnsAppInitError проверяет, что ошибка инициализации приложения возвращается вызывающему коду.
func Test_run_ReturnsAppInitError(t *testing.T) {
	// Arrange
	cfg := config.ServerConfig{DatabaseURI: "://bad-database-uri"}
	logger := logging.NopLogger()

	// Act
	err := run(cfg, logger)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "app init error")
}
